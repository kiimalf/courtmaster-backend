package service

import (
	"log/slog"

	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

type NotificationService struct {
	notifRepo  repository.NotificationRepository
	outboxRepo repository.OutboxRepository
	relay      *OutboxRelay
	inspector  *asynq.Inspector
	logger     *slog.Logger
}

func NewNotificationService(
	notifRepo repository.NotificationRepository,
	outboxRepo repository.OutboxRepository,
	relay *OutboxRelay,
	inspector *asynq.Inspector,
	logger *slog.Logger,
) *NotificationService {
	return &NotificationService{
		notifRepo:  notifRepo,
		outboxRepo: outboxRepo,
		relay:      relay,
		inspector:  inspector,
		logger:     logger,
	}
}

// ListNotifications godoc
// @Summary Ambil daftar notifikasi milik pengguna yang sedang login
// @Security BearerAuth
// @Tags Notifications
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /notifications [get]
func (s *NotificationService) ListNotifications(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	list, err := s.notifRepo.FindByUserID(ctx, user.ID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar notifikasi berhasil diambil", list)
}

// RetryOutbox godoc
// @Summary Trigger manual relay event outbox yang tertunda (Admin Only)
// @Security BearerAuth
// @Tags Notifications
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /notifications/outbox/retry [post]
func (s *NotificationService) RetryOutbox(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	count, err := s.relay.PublishPending(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "relay outbox berhasil dijalankan", map[string]int{"published": count})
}

// GetQueueStats godoc
// @Summary Monitoring statistik antrean Asynq dan outbox event (Admin Only)
// @Security BearerAuth
// @Tags Admin
// @Produce json
// @Success 200 {object} model.WebResponse
// @Router /admin/queue [get]
func (s *NotificationService) GetQueueStats(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	pendingOutbox, err := s.outboxRepo.CountPending(ctx)
	if err != nil {
		return helper.Internal(err)
	}

	queueInfo, err := s.inspector.GetQueueInfo("default")
	if err != nil {
		// Redis sempat tidak tersedia: tetap kembalikan statistik outbox SQL.
		s.logger.Warn("gagal_mengambil_info_antrean_redis", slog.String("error", err.Error()))
		return helper.Success(c, fiber.StatusOK, "statistik antrean parsial", map[string]any{
			"pending_outbox": pendingOutbox,
			"asynq_queue":    nil,
		})
	}

	return helper.Success(c, fiber.StatusOK, "statistik antrean berhasil diambil", map[string]any{
		"pending_outbox": pendingOutbox,
		"asynq_queue": map[string]any{
			"active":    queueInfo.Active,
			"pending":   queueInfo.Pending,
			"scheduled": queueInfo.Scheduled,
			"retry":     queueInfo.Retry,
			"archived":  queueInfo.Archived,
			"completed": queueInfo.Completed,
			"size":      queueInfo.Size,
		},
	})
}
