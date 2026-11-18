package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"courtmaster-backend/app/repository"
	"courtmaster-backend/app/service"
	"courtmaster-backend/config"
	"courtmaster-backend/database"
	_ "courtmaster-backend/docs"
	"courtmaster-backend/helper"
	"courtmaster-backend/route"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

const minSecretLength = 32

// @title CourtMaster Backend API
// @version 1.0
// @description Sistem Backend Reservasi Gelanggang Olahraga & Matchmaking Komunitas
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	config.LoadEnv()
	logger := config.NewLogger()

	role := flag.String("role", config.GetEnv("APP_ROLE", "all"), "api | worker | all")
	flag.Parse()

	runAPI := *role == "api" || *role == "all"
	runWorker := *role == "worker" || *role == "all"
	if !runAPI && !runWorker {
		logger.Error("Role tidak dikenal", slog.String("role", *role))
		os.Exit(1)
	}

	if *role == "worker" {
		startWorker(logger)
		return
	}

	pool, err := database.NewPool(context.Background())
	if err != nil {
		logger.Error("Gagal terhubung ke PostgreSQL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var app *fiber.App
	if runAPI {
		app = startAPI(logger, pool)
		if app == nil {
			os.Exit(1)
		}
	}

	if runWorker {
		go startWorker(logger)
	}

	logger.Info("Aplikasi berjalan", slog.String("role", *role))
	<-ctx.Done()

	if app != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			logger.Error("Shutdown HTTP tidak rapi", slog.String("error", err.Error()))
		}
	}

	logger.Info("Aplikasi berhenti dengan rapi")
}

func startAPI(logger *slog.Logger, pool *pgxpool.Pool) *fiber.App {
	jwtSecret := config.GetEnv("JWT_SECRET", "")
	if len(jwtSecret) < minSecretLength {
		logger.Error("JWT_SECRET tidak diisi atau terlalu pendek",
			slog.Int("minimal_karakter", minSecretLength))
		return nil
	}

	mongoClient, err := database.NewMongoClient(context.Background())
	if err != nil {
		logger.Error("Gagal terhubung ke MongoDB", slog.String("error", err.Error()))
		return nil
	}
	mongoDB := mongoClient.Database(config.GetEnv("MONGO_DATABASE", "courtmaster_mongo"))

	txManager := repository.NewTxManager(pool)
	courtRepo := repository.NewCourtRepository(pool)
	roleRepo := repository.NewRoleRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	tokenRepo := repository.NewTokenRepository(pool)
	reviewRepo := repository.NewReviewRepository(mongoDB)
	bookingRepo := repository.NewBookingRepository(pool)
	outboxRepo := repository.NewOutboxRepository(pool)
	notifRepo := repository.NewNotificationRepository(pool)

	redisOpt := asynq.RedisClientOpt{
		Addr:     config.GetEnv("REDIS_ADDR", "localhost:6379"),
		Password: config.GetEnv("REDIS_PASSWORD", ""),
		DB:       config.GetEnvInt("REDIS_DB", 0),
	}
	asynqClient := asynq.NewClient(redisOpt)
	asynqInspector := asynq.NewInspector(redisOpt)

	jwtManager := helper.NewJWTManager(
		jwtSecret,
		15*time.Minute,
		168*time.Hour,
	)

	rawPermissions, err := roleRepo.LoadPermissions(context.Background())
	if err != nil {
		logger.Error("Gagal memuat permission", slog.String("error", err.Error()))
		return nil
	}
	permSet := helper.NewPermissionSet(rawPermissions)
	logger.Info("Permission dimuat", slog.Any("roles", permSet.KnownRoles()))

	courtService := service.NewCourtService(courtRepo, logger)
	authService := service.NewAuthService(userRepo, tokenRepo, jwtManager, logger)
	reviewService := service.NewReviewService(reviewRepo, bookingRepo, logger)
	outboxRelay := service.NewOutboxRelay(outboxRepo, asynqClient, logger)
	bookingService := service.NewBookingService(bookingRepo, courtRepo, outboxRepo, txManager, logger)
	notificationService := service.NewNotificationService(notifRepo, outboxRepo, outboxRelay, asynqInspector, logger)

	app := config.NewApp(logger)
	route.Register(app, route.Dependencies{
		Pool:                pool,
		Mongo:               mongoClient,
		JWT:                 jwtManager,
		Permissions:         permSet,
		CourtService:        courtService,
		AuthService:         authService,
		ReviewService:       reviewService,
		BookingService:      bookingService,
		NotificationService: notificationService,
	})

	port := config.GetEnv("APP_PORT", "8080")
	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Error("Server berhenti", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()
	logger.Info("HTTP berjalan", slog.String("port", port))

	return app
}
