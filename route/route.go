package route

import (
	"context"
	"time"

	"courtmaster-backend/app/service"
	"courtmaster-backend/helper"
	"courtmaster-backend/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Dependencies struct {
	Pool                *pgxpool.Pool
	Mongo               *mongo.Client
	JWT                 *helper.JWTManager
	Permissions         *helper.PermissionSet
	CourtService        *service.CourtService
	AuthService         *service.AuthService
	ReviewService       *service.ReviewService
	BookingService      *service.BookingService
	NotificationService *service.NotificationService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")

	perms := deps.Permissions

	// Healthcheck
	api.Get("/health", healthCheck(deps.Pool, deps.Mongo))

	// Auth
	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login",
		middleware.LoginRateLimit(),
		deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout",
		middleware.RequireAuth(deps.JWT),
		deps.AuthService.Logout)
	auth.Get("/me",
		middleware.RequireAuth(deps.JWT),
		deps.AuthService.Me)

	// Courts
	courts := api.Group("/courts")
	courts.Get("/", deps.CourtService.ListCourts)
	courts.Get("/:id", deps.CourtService.GetCourtByID)
	courts.Get("/:id/slots", deps.CourtService.ListSlots)
	courts.Post("/",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "court:create"),
		deps.CourtService.CreateCourt)
	courts.Patch("/:id/rate",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "court:manage"),
		deps.CourtService.UpdateHourlyRate)
	courts.Patch("/:id",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "court:manage"),
		deps.CourtService.UpdateHourlyRate)

	// Court Reviews
	courts.Get("/:id/reviews", deps.ReviewService.ListReviews)
	courts.Get("/:id/reviews/summary", deps.ReviewService.GetCourtSummary)
	courts.Post("/:id/reviews",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "review:create"),
		deps.ReviewService.CreateReview)
	courts.Post("/:id/reviews/:reviewId/helpful",
		middleware.RequireAuth(deps.JWT),
		deps.ReviewService.VoteHelpful)
	courts.Post("/:id/reviews/:reviewId/replies",
		middleware.RequireAuth(deps.JWT),
		deps.ReviewService.AddReply)

	// Reviews
	reviews := api.Group("/reviews", middleware.RequireAuth(deps.JWT))
	reviews.Post("/:id/helpful", deps.ReviewService.VoteHelpful)
	reviews.Post("/:id/replies", deps.ReviewService.AddReply)

	// Bookings
	bookings := api.Group("/bookings", middleware.RequireAuth(deps.JWT))
	bookings.Post("/",
		middleware.RequirePermission(perms, "booking:create"),
		deps.BookingService.CreateBooking)
	bookings.Get("/",
		middleware.RequirePermission(perms, "booking:read"),
		deps.BookingService.ListUserBookings)
	bookings.Get("/my",
		middleware.RequirePermission(perms, "booking:read"),
		deps.BookingService.ListUserBookings)
	bookings.Post("/swap",
		middleware.RequirePermission(perms, "booking:update"),
		deps.BookingService.SwapBooking)
	bookings.Get("/export",
		middleware.RequirePermission(perms, "booking:export"),
		deps.BookingService.ListUserBookings)

	// Sparrings
	sparrings := api.Group("/sparrings")
	sparrings.Get("/", deps.BookingService.ListSparrings)
	sparrings.Post("/",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "sparring:create"),
		deps.BookingService.CreateSparring)
	sparrings.Post("/:id/join",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "sparring:join"),
		deps.BookingService.JoinSparring)

	// Notification & Queue
	notifications := api.Group("/notifications", middleware.RequireAuth(deps.JWT))
	notifications.Get("/mine", deps.NotificationService.ListNotifications)
	notifications.Get("/", deps.NotificationService.ListNotifications)
	notifications.Post("/outbox/retry",
		middleware.RequirePermission(perms, "outbox:manage"),
		deps.NotificationService.RetryOutbox)

	admin := api.Group("/admin",
		middleware.RequireAuth(deps.JWT),
		middleware.RequirePermission(perms, "queue:monitor"))
	admin.Get("/queue", deps.NotificationService.GetQueueStats)

	app.Use(func(c *fiber.Ctx) error {
		return helper.NotFound("Endpoint tidak ditemukan")
	})
}

func healthCheck(pool *pgxpool.Pool, mongoClient *mongo.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return helper.ServiceUnavailable("PostgreSQL tidak dapat dihubungi")
		}
		if err := mongoClient.Ping(ctx, nil); err != nil {
			return helper.ServiceUnavailable("MongoDB tidak dapat dihubungi")
		}
		return helper.Success(c, fiber.StatusOK, "Server, PostgreSQL, dan MongoDB berjalan prima", nil)
	}
}
