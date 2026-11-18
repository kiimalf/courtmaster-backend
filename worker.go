package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"courtmaster-backend/app/repository"
	"courtmaster-backend/app/service"
	"courtmaster-backend/config"
	"courtmaster-backend/database"

	"github.com/hibiken/asynq"
)

// startWorker menjalankan proses worker Asynq (consumer notifikasi) sekaligus
// background relay outbox. Dipanggil dari main.go ketika `-role=worker|all`.
//
// Catatan untuk Git Master: file ini sengaja tidak mendefinisikan main() dan
// tidak mengubah main.go. Parsing flag `-role=api|worker|all` sepenuhnya
// menjadi tanggung jawab main.go saat wiring Tahap 2.
func startWorker(logger *slog.Logger) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewPool(ctx)
	if err != nil {
		logger.Error("worker_db_pool_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.GetEnv("REDIS_ADDR", "localhost:6379"),
		Password: config.GetEnv("REDIS_PASSWORD", ""),
		DB:       config.GetEnvInt("REDIS_DB", 0),
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: config.GetEnvInt("WORKER_CONCURRENCY", 5),
		},
	)

	txManager := repository.NewTxManager(pool)
	workerHandler := service.NewNotificationWorker(pool, txManager, logger)

	mux := asynq.NewServeMux()
	mux.HandleFunc(service.TaskTypeSendNotification, workerHandler.HandleSendNotification)

	// Outbox relay berjalan sebagai background runner di proses worker.
	outboxRepo := repository.NewOutboxRepository(pool)
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()
	relay := service.NewOutboxRelay(outboxRepo, asynqClient, logger)
	go relay.Run(ctx)

	if err := srv.Start(mux); err != nil {
		logger.Error("asynq_worker_start_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logger.Info("asynq_worker_started", slog.String("redis", redisOpt.Addr))

	<-ctx.Done()
	logger.Info("asynq_worker_shutting_down")
	srv.Shutdown()
}
