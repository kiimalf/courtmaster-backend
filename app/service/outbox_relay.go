package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/config"

	"github.com/hibiken/asynq"
)

// TaskTypeSendNotification adalah nama task Asynq yang dikonsumsi NotificationWorker.
const TaskTypeSendNotification = "booking:send_notification"

const (
	defaultRelayInterval = 2 * time.Second
	defaultRelayBatch    = 50
)

type OutboxRelay struct {
	repo   repository.OutboxRepository
	client *asynq.Client
	logger *slog.Logger
}

func NewOutboxRelay(repo repository.OutboxRepository, client *asynq.Client, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{repo: repo, client: client, logger: logger}
}

// Run menjalankan loop background relay berkala (Modul 11). Loop langsung
// mengeksekusi satu batch saat start agar event tidak menunggu interval pertama.
func (r *OutboxRelay) Run(ctx context.Context) {
	interval := time.Duration(config.GetEnvInt("RELAY_INTERVAL_MS", int(defaultRelayInterval/time.Millisecond))) * time.Millisecond
	if interval <= 0 {
		interval = defaultRelayInterval
	}

	relayOnce := func() {
		count, err := r.PublishPending(ctx)
		if err != nil {
			r.logger.Error("outbox_relay_error", slog.String("error", err.Error()))
			return
		}
		if count > 0 {
			r.logger.Info("outbox_auto_relayed", slog.Int("count", count))
		}
	}

	relayOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			relayOnce()
		}
	}
}

// PublishPending mengambil batch event yang belum dipublikasikan lalu meng-enqueue
// task Asynq dengan Idempotency Key `outbox-<id>`.
//
// Keamanan batch:
//   - Asynq menolak TaskID duplikat (ErrTaskIDConflict), sehingga relay paralel
//     tidak dapat membuat task ganda untuk event yang sama.
//   - Consumer tetap melakukan klaim idempotent lewat UNIQUE(outbox_id).
//   - Event hanya ditandai published setelah enqueue berhasil (atau sudah ada).
func (r *OutboxRelay) PublishPending(ctx context.Context) (int, error) {
	batchSize := config.GetEnvInt("RELAY_BATCH", defaultRelayBatch)
	if batchSize < 1 {
		batchSize = defaultRelayBatch
	}

	events, err := r.repo.FetchUnpublished(ctx, batchSize)
	if err != nil {
		return 0, err
	}

	publishedCount := 0
	for _, event := range events {
		job := model.NotificationJob{
			OutboxID:  event.ID,
			EventType: event.EventType,
			Payload:   event.Payload,
		}
		jobBytes, err := json.Marshal(job)
		if err != nil {
			r.logger.Error("failed_to_serialize_outbox_job",
				slog.Int64("outbox_id", event.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		task := asynq.NewTask(TaskTypeSendNotification, jobBytes,
			asynq.TaskID(fmt.Sprintf("outbox-%d", event.ID)),
			asynq.Retention(24*time.Hour),
			asynq.MaxRetry(3),
		)

		if _, err := r.client.EnqueueContext(ctx, task); err != nil {
			// TaskID sudah terdaftar berarti event ini sudah pernah di-enqueue.
			if !errors.Is(err, asynq.ErrTaskIDConflict) {
				r.logger.Error("failed_to_enqueue_outbox_task",
					slog.Int64("outbox_id", event.ID),
					slog.String("error", err.Error()),
				)
				continue
			}
			r.logger.Info("outbox_task_already_enqueued", slog.Int64("outbox_id", event.ID))
		}

		if err := r.repo.MarkPublished(ctx, event.ID); err != nil {
			// Event sudah terlanjur masuk Redis; biarkan tetap unpublished agar
			// percobaan relay berikutnya hanya menghasilkan TaskID conflict.
			r.logger.Error("failed_to_mark_outbox_published",
				slog.Int64("outbox_id", event.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		publishedCount++
	}

	return publishedCount, nil
}
