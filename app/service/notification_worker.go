package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NotificationWorker struct {
	pool      *pgxpool.Pool
	txManager *repository.TxManager
	notifRepo repository.NotificationRepository
	logger    *slog.Logger
}

func NewNotificationWorker(pool *pgxpool.Pool, txManager *repository.TxManager, logger *slog.Logger) *NotificationWorker {
	return &NotificationWorker{
		pool:      pool,
		txManager: txManager,
		notifRepo: repository.NewNotificationRepository(pool),
		logger:    logger,
	}
}

// HandleSendNotification adalah consumer Asynq yang memproses event outbox.
// Idempotensi dijaga oleh UNIQUE(outbox_id) di tabel notifications, sehingga
// retry Asynq tidak akan mengirim notifikasi ganda.
func (w *NotificationWorker) HandleSendNotification(ctx context.Context, t *asynq.Task) error {
	var job model.NotificationJob
	if err := json.Unmarshal(t.Payload(), &job); err != nil {
		// Payload rusak tidak akan membaik dengan retry.
		return fmt.Errorf("%w: payload task rusak: %v", asynq.SkipRetry, err)
	}

	switch job.EventType {
	case model.EventTypeBookingCreated:
		return w.handleBookingCreated(ctx, job)
	default:
		w.logger.Warn("unknown_outbox_event_skipped",
			slog.Int64("outbox_id", job.OutboxID),
			slog.String("event_type", job.EventType),
		)
		return nil
	}
}

func (w *NotificationWorker) handleBookingCreated(ctx context.Context, job model.NotificationJob) error {
	var payload model.BookingCreatedPayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("%w: payload booking rusak: %v", asynq.SkipRetry, err)
	}

	notif := model.Notification{
		OutboxID: job.OutboxID,
		UserID:   payload.UserID,
		Channel:  "EMAIL",
		Subject:  "Konfirmasi Pemesanan Lapangan " + payload.BookingCode,
		Body:     fmt.Sprintf("Booking Anda sebesar Rp %.2f untuk tanggal %s berhasil dikonfirmasi.", payload.TotalAmount, payload.Date),
	}

	var claimed bool
	err := w.txManager.WithTx(ctx, func(tx repository.DBTX) error {
		var err error
		claimed, err = w.notifRepo.Claim(ctx, tx, &notif)
		return err
	})
	if err != nil {
		return err
	}
	if !claimed {
		w.logger.Info("idempotent_duplicate_notification_skipped", slog.Int64("outbox_id", job.OutboxID))
		return nil
	}

	// Di lingkungan nyata, pemanggilan SMTP/email provider diletakkan di sini.
	w.logger.Info("notification_sent_successfully",
		slog.Int64("outbox_id", job.OutboxID),
		slog.String("booking_code", payload.BookingCode),
	)
	return nil
}
