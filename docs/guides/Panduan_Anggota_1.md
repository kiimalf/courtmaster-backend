# PANDUAN PENGERJAAN — ANGGOTA 1
## Peran: Git Master, DevOps Engineer & Court Management Domain Lead
### Sistem: CourtMaster (Smart Sports Arena & Community Matchmaking)

---

## 1. Ikhtisar Tugas & Tanggung Jawab

Sebagai **Anggota 1**, Anda memegang peran ganda yang sangat penting bagi keberhasilan tim:
1. **DevOps & Git Master (Tahap 0 & Tahap 2)**:
   - **Tahap 0 (Foundation Freeze)**: Menyiapkan base project di branch `develop`, mengunduh **seluruh dependensi proyek sekaligus** (Modul 1–12), menyalin seluruh file `helper/` secara lengkap, menyiapkan `config/`, `database/`, dan skema basis data awal fondasi otorisasi & pengguna (`001_roles_and_permissions.sql` dan `002_users.sql`). Setiap komponen di-*commit* secara granular (**Micro-Commits 1 s.d. 7**).
   - **Tahap 2 (Integrasi & Wiring)**: Me-review Pull Request (PR) dari Anggota 2 dan Anggota 3, melakukan merge ke branch `develop`, menyambungkan seluruh rute ke dalam file tunggal `route/route.go` (**Pola Opsi B**), me-wire `main.go`, menguji E2E, serta membangun *multi-stage* `Dockerfile` dan `docker-compose.yml` (**Micro-Commits 12 s.d. 14**).
2. **Domain Developer (Tahap 1)**:
   - Mengerjakan fitur Court Management pada branch terpisah: **`feat/court-management`** (**Micro-Commits 8 s.d. 11**, lalu buka PR #1 ke `develop`).
   - Membuat skema DDL gelanggang dan slot jadwal (`003_courts_and_slots.sql`), CRUD Gelanggang/Lapangan Olahraga, jadwal ketersediaan slot jam, dan mekanisme proteksi konkurensi **Optimistic Locking** (`version`) sesuai Modul 9.
   - Menulis unit test *table-driven* untuk business rules Court (`court_rules_test.go`).

---

## 2. Struktur Direktori Proyek Lengkap (Target Akhir Repositori)

```
courtmaster-backend/
├── app/
│   ├── model/
│   │   ├── auth_user.go              # Model konteks autentikasi global
│   │   ├── booking.go                # Model Booking awal (untuk CSV export & isolasi domain)
│   │   ├── court.go                  # Model Court & CourtSlot
│   │   ├── cursor.go                 # Model Keyset Cursor
│   │   └── web_response.go           # Response envelope baku
│   ├── repository/
│   │   ├── court_repository.go       # Interface & PostgreSQL query court
│   │   ├── dbtx.go                   # Abstraksi DBTX Modul 9 (codeUniqueViolation, pgErrorCode)
│   │   ├── errors.go                 # Sentinel error global repository (ErrNotFound, ErrConflict)
│   │   ├── role_repository.go        # Loading in-memory RBAC permissions dari database
│   │   └── tx.go                     # ACID TxManager auto-retry 40001/40P01
│   └── service/
│       ├── court_rules.go            # Pure business rules (tanpa driver/Fiber)
│       ├── court_rules_test.go       # Table-driven unit test
│       └── court_service.go          # Fiber HTTP handler methods
├── config/
│   ├── app.go                        # Fiber App & Centralized Error Handler
│   ├── env.go                        # Helper GetEnv & GetEnvInt
│   └── logger.go                     # Slog JSON Logger & Lumberjack
├── database/
│   ├── mongo.go                      # MongoDB client & Ping healthcheck
│   └── postgres.go                   # pgxpool NewPool connection
├── helper/                           # LENGKAP 100% (FROZEN UNTUK ANGGOTA 2 & 3)
│   ├── authz.go                      # In-memory RBAC PermissionSet
│   ├── context.go                    # CurrentUser & LocalsAuthUser
│   ├── cursor.go                     # Base64 Keyset Cursor Pagination
│   ├── errors.go                     # Centralized Error Factory (Lengkap)
│   ├── jwt.go                        # JWTManager Generate & Verify Token
│   ├── negotiate.go                  # Content Negotiation JSON/CSV
│   ├── request.go                    # RequestContext (5s) & ParamID
│   ├── response.go                   # Success, Created, SuccessList, SuccessCursor WebResponse
│   ├── security.go                   # HashPassword & Timing Attack Defense
│   └── validator.go                  # Go-Playground Validator (custom tags & messages)
├── middleware/                       # Dibuat oleh Anggota 2 pada Tahap 1
│   ├── auth.go                       # RequireAuth & RequireJSON
│   ├── authz.go                      # RequirePermission RBAC
│   └── ratelimit.go                  # LoginRateLimit
├── migrations/
│   ├── 001_roles_and_permissions.sql # Roles, Permissions & RBAC seed (Tahap 0)
│   ├── 002_users.sql                 # Users table & Default Accounts seed (Tahap 0)
│   └── 003_courts_and_slots.sql      # DDL Courts & Slots schema with seed (Tahap 1 - PR #1)
├── docs/                             # Swagger OpenAPI Generated Files
├── Dockerfile                        # Multi-stage build (golang:1.22-alpine -> scratch)
├── docker-compose.yml                # 5-Service orchestration (api, worker, postgres, mongo, redis)
├── Makefile                          # Command shortcuts
├── .env.example                      # Kamus baku konfigurasi tim
├── .gitignore                        # Standard Go gitignore
├── go.mod                            # Berisi SELURUH dependencies Modul 1-12
├── go.sum
├── tools.go                          # Dependency anchor (mengunci asynq, swagger, swag agar tidak ter-prune go mod tidy)
├── main.go                           # Entry point + Graceful Shutdown (CLI role router)
├── worker.go                         # Asynq background worker runner (dibuat Anggota 3 pada PR #5)
└── route/
    └── route.go                      # File tunggal registrasi rute API + fallback 404
```

---

## 3. Rangkaian Kerja Anggota 1 — TAHAP 0 (Base Foundation Setup)

Jalankan rangkaian micro-commits berikut secara berurutan di branch `develop`. Setiap langkah hanya men-stage file yang relevan agar riwayat Git terstruktur rapi.

```bash
mkdir -p courtmaster-backend && cd courtmaster-backend
go mod init courtmaster-backend
git init -b develop
```

---

### Micro-Commit 1: Inisialisasi Repositori, Dependensi & Environment Template

Unduh seluruh dependensi proyek (Modul 1–12) sekaligus agar `go.mod` dan `go.sum` langsung beku (*frozen*):

```bash
go get github.com/gofiber/fiber/v2@v2.52.4
go get github.com/jackc/pgx/v5@v5.6.0
go get github.com/joho/godotenv@v1.5.1
go get gopkg.in/natefinch/lumberjack.v2@v2.2.1
go get github.com/gofiber/swagger@v1.1.1
go get github.com/swaggo/swag@v1.16.6
go get github.com/go-playground/validator/v10@v10.22.0
go get github.com/golang-jwt/jwt/v5@v5.2.1
go get golang.org/x/crypto@v0.25.0
go get go.mongodb.org/mongo-driver/v2@v2.0.0-beta1
go get github.com/redis/go-redis/v9@v9.6.1
go get github.com/hibiken/asynq@v0.24.1
```

Buat file **`tools.go`** (Pola resmi Go untuk mengunci dependensi tooling/asinkron agar tidak terhapus otomatis oleh `go mod tidy`):
```go
//go:build tools
package tools

import (
	_ "github.com/gofiber/swagger"
	_ "github.com/hibiken/asynq"
	_ "github.com/swaggo/swag"
)
```

Jalankan `go mod tidy` untuk menyelaraskan `go.mod` dan `go.sum`:
```bash
go mod tidy
```

Buat file **`.gitignore`**:
```gitignore
.env
.env.*
logs/
bin/
tmp/
*.exe
*.test
```

Buat file **`.env.example`**:
```env
APP_NAME="CourtMaster Backend API"
APP_PORT=8080
LOG_LEVEL=info
ALLOWED_ORIGINS=http://localhost:5173

# Database PostgreSQL
DB_HOST=localhost
DB_PORT=5432
DB_USER=courtmaster
DB_PASSWORD=secret
DB_NAME=courtmaster_db
DB_SSLMODE=disable
DB_MAX_CONNS=10

# Database MongoDB
MONGO_URI=mongodb://localhost:27017
MONGO_DATABASE=courtmaster_mongo
MONGO_MAX_POOL=20

# Keamanan & JWT
JWT_SECRET=supersecretkey_courtmaster_min32chars_demo!
JWT_ISSUER=courtmaster-backend
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_DAYS=7

# Redis & Asynq Worker (Modul 11)
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

WORKER_CONCURRENCY=5
RELAY_INTERVAL_MS=2000
RELAY_BATCH=50

# Dokumentasi
SWAGGER_ENABLED=true
```

```bash
git add go.mod go.sum tools.go .gitignore .env.example
git commit -m "chore(setup): initialize go module, download full dependencies, and env template"
```

---

### Micro-Commit 2: Konfigurasi Lingkungan & JSON Structured Logging

**`config/env.go`**:
```go
package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("Peringatan: berkas .env tidak ditemukan, memakai environment sistem")
	}
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func GetEnvInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Peringatan: %s bukan angka (%q), memakai bawaan %d", key, value, fallback)
		return fallback
	}
	return parsed
}
```

**`config/logger.go`**:
```go
package config

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLogger() *slog.Logger {
	if err := os.MkdirAll("logs", 0o755); err != nil {
		panic("Gagal membuat folder logs: " + err.Error())
	}

	rotator := &lumberjack.Logger{
		Filename:   filepath.Join("logs", "app.log"),
		MaxSize:    10, // MB
		MaxBackups: 5,
		MaxAge:     14, // Days
		Compress:   true,
	}

	writer := io.MultiWriter(os.Stdout, rotator)
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: parseLevel(GetEnv("LOG_LEVEL", "info")),
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

func parseLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
```

```bash
git add config/env.go config/logger.go
git commit -m "chore(config): implement environment reader and structured JSON logger"
```

---

### Micro-Commit 3: Koneksi Database (`pgxpool` & `mongo`) serta Abstraksi `TxManager`

**`database/postgres.go`**:
```go
package database

import (
	"context"
	"fmt"
	"time"

	"courtmaster-backend/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		config.GetEnv("DB_USER", "courtmaster"),
		config.GetEnv("DB_PASSWORD", "secret"),
		config.GetEnv("DB_HOST", "localhost"),
		config.GetEnv("DB_PORT", "5432"),
		config.GetEnv("DB_NAME", "courtmaster_db"),
		config.GetEnv("DB_SSLMODE", "disable"),
	)

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("konfigurasi database tidak valid: %w", err)
	}

	cfg.MaxConns = int32(config.GetEnvInt("DB_MAX_CONNS", 10))
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("gagal memuat pgxpool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("gagal terhubung ke postgres: %w", err)
	}

	return pool, nil
}
```

**`database/mongo.go`**:
```go
package database

import (
	"context"
	"fmt"
	"time"

	"courtmaster-backend/config"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func NewMongoClient(ctx context.Context) (*mongo.Client, error) {
	opts := options.Client().
		ApplyURI(config.GetEnv("MONGO_URI", "mongodb://localhost:27017")).
		SetServerSelectionTimeout(5 * time.Second).
		SetMaxPoolSize(uint64(config.GetEnvInt("MONGO_MAX_POOL", 20)))

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan client MongoDB: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("ping MongoDB: %w", err)
	}

	return client, nil
}
```

**`app/repository/dbtx.go`**:
```go
package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	codeUniqueViolation      = "23505"
	codeSerializationFailure = "40001"
	codeDeadlockDetected     = "40P01"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
```

**`app/repository/tx.go`**:
```go
package repository

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TxManager struct {
	pool        *pgxpool.Pool
	maxAttempts int
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool, maxAttempts: 3}
}

func (m *TxManager) WithTx(ctx context.Context, fn func(tx DBTX) error) error {
	var err error
	for attempt := 1; attempt <= m.maxAttempts; attempt++ {
		err = m.runOnce(ctx, fn)
		if !isRetryable(err) {
			return err
		}

		wait := time.Duration(attempt*20+rand.IntN(30)) * time.Millisecond
		select {
		case <-ctx.Done():
			return err
		case <-time.After(wait):
		}
	}
	return err
}

func (m *TxManager) runOnce(ctx context.Context, fn func(tx DBTX) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("memulai transaction: %w", err)
	}

	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func isRetryable(err error) bool {
	code := pgErrorCode(err)
	return code == codeSerializationFailure || code == codeDeadlockDetected
}
```

**`app/repository/errors.go`**:
```go
package repository

import "errors"

var (
	ErrNotFound = errors.New("data tidak ditemukan")
	ErrConflict = errors.New("konflik data atau versi telah berubah")
)
```

```bash
git add database/postgres.go database/mongo.go app/repository/dbtx.go app/repository/tx.go app/repository/errors.go
git commit -m "feat(database): implement pgxpool, mongo client, ACID TxManager, and repository sentinel errors"
```

---

### Micro-Commit 4: Model Inti WebResponse, AuthUser, Cursor & Booking Model

**`app/model/web_response.go`**:
```go
package model

type WebResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

type ErrorResponse struct {
	Success   bool              `json:"success"`
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

type ListQuery struct {
	Page     int
	Limit    int
	Search   string
	Sort     string
	Order    string
	IsActive *bool
}

func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}

type CursorQuery struct {
	Limit    int
	Search   string
	IsActive *bool
	After    *Cursor
}

type CursorMeta struct {
	Limit      int    `json:"limit"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}
```

**`app/model/auth_user.go`**:
```go
package model

type AuthUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	RoleID   int    `json:"role_id"`
	RoleName string `json:"role_name"`
}
```

**`app/model/cursor.go`**:
```go
package model

import "time"

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        int64     `json:"id"`
}
```

**`app/model/booking.go`**:
```go
package model

import "time"

type Booking struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	CourtID     int64     `json:"court_id"`
	BookingCode string    `json:"booking_code"`
	Date        string    `json:"date"`
	TotalAmount float64   `json:"total_amount"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}
```

```bash
git add app/model/web_response.go app/model/auth_user.go app/model/cursor.go app/model/booking.go
git commit -m "feat(model): define WebResponse envelope, global AuthUser, Cursor, and Booking entities"
```

---

### Micro-Commit 5: Paket Helper Inti (Errors, Response, Request, Validator, Context & Keyset Cursor)

Buat file-file helper inti dengan kode baku berikut (sehingga Anggota 2 & 3 tidak perlu memodifikasi helper lagi):

**`helper/errors.go`**:
```go
package helper

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

const (
	CodeValidation       = "VALIDATION_ERROR"
	CodeBadRequest       = "BAD_REQUEST"
	CodeUnauthorized     = "UNAUTHORIZED"
	CodeForbidden        = "FORBIDDEN"
	CodeNotFound         = "NOT_FOUND"
	CodeConflict         = "CONFLICT"
	CodeUnsupportedMedia = "UNSUPPORTED_MEDIA_TYPE"
	CodeNotAcceptable    = "NOT_ACCEPTABLE"
	CodeTooManyRequests  = "TOO_MANY_REQUESTS"
	CodeInternal         = "INTERNAL_ERROR"
	CodeService          = "SERVICE_UNAVAILABLE"
)

type AppError struct {
	Status  int               `json:"-"`
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"errors,omitempty"`
	cause   error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.cause
}

func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

func BadRequest(message string) *AppError {
	return &AppError{Status: fiber.StatusBadRequest, Code: CodeBadRequest, Message: message}
}

func Unauthorized(message string) *AppError {
	return &AppError{Status: fiber.StatusUnauthorized, Code: CodeUnauthorized, Message: message}
}

func Forbidden(message string) *AppError {
	return &AppError{Status: fiber.StatusForbidden, Code: CodeForbidden, Message: message}
}

func NotFound(message string) *AppError {
	return &AppError{Status: fiber.StatusNotFound, Code: CodeNotFound, Message: message}
}

func Conflict(message string) *AppError {
	return &AppError{Status: fiber.StatusConflict, Code: CodeConflict, Message: message}
}

func Validation(fields map[string]string) *AppError {
	return &AppError{
		Status:  fiber.StatusUnprocessableEntity,
		Code:    CodeValidation,
		Message: "validasi gagal",
		Fields:  fields,
	}
}

func UnsupportedMediaType(message string) *AppError {
	return &AppError{Status: fiber.StatusUnsupportedMediaType, Code: CodeUnsupportedMedia, Message: message}
}

func TooManyRequests(message string) *AppError {
	return &AppError{Status: fiber.StatusTooManyRequests, Code: CodeTooManyRequests, Message: message}
}

func ServiceUnavailable(message string) *AppError {
	return &AppError{Status: fiber.StatusServiceUnavailable, Code: CodeService, Message: message}
}

func NotAcceptable(message string) *AppError {
	return &AppError{Status: fiber.StatusNotAcceptable, Code: CodeNotAcceptable, Message: message}
}

func Internal(cause error) *AppError {
	return &AppError{
		Status:  fiber.StatusInternalServerError,
		Code:    CodeInternal,
		Message: "terjadi kesalahan internal pada server",
		cause:   cause,
	}
}
```

**`helper/response.go`**:
```go
package helper

import (
	"courtmaster-backend/app/model"
	"github.com/gofiber/fiber/v2"
)

func Success(c *fiber.Ctx, status int, message string, data any) error {
	return c.Status(status).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func SuccessList(c *fiber.Ctx, message string, data any, meta *model.Meta) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

func Created(c *fiber.Ctx, message string, data any, location string) error {
	if location != "" {
		c.Set("Location", location)
	}
	return c.Status(fiber.StatusCreated).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func SuccessCursor(c *fiber.Ctx, message string, data any, meta *model.CursorMeta) error {
	return c.Status(fiber.StatusOK).JSON(model.WebResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}
```

**`helper/request.go`**:
```go
package helper

import (
	"context"
	"strconv"
	"strings"
	"time"

	"courtmaster-backend/app/model"
	"github.com/gofiber/fiber/v2"
)

func RequestContext(c *fiber.Ctx) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.UserContext(), 5*time.Second)
}

// ParamID mengekstrak parameter ID dari URL (default ":id" atau nama kustom) dan mengubahnya menjadi int64
func ParamID(c *fiber.Ctx, paramName ...string) (int64, bool) {
	name := "id"
	if len(paramName) > 0 && paramName[0] != "" {
		name = paramName[0]
	}
	id, err := strconv.ParseInt(c.Params(name), 10, 64)
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

func ParseListQuery(c *fiber.Ctx) model.ListQuery {
	q := model.ListQuery{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 10),
		Search: strings.TrimSpace(c.Query("search")),
		Sort:   c.Query("sort", "id"),
		Order:  strings.ToLower(c.Query("order", "asc")),
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 {
		q.Limit = 1
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Order != "desc" {
		q.Order = "asc"
	}

	if raw := c.Query("is_active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			q.IsActive = &v
		}
	}

	return q
}

func ParseCursorQuery(c *fiber.Ctx) (model.CursorQuery, error) {
	limit := c.QueryInt("limit", 10)
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	q := model.CursorQuery{
		Limit:  limit,
		Search: strings.TrimSpace(c.Query("search")),
	}

	if raw := c.Query("is_active"); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			q.IsActive = &v
		} else {
			return q, BadRequest("is_active harus berupa boolean")
		}
	}

	if rawCursor := strings.TrimSpace(c.Query("cursor")); rawCursor != "" {
		cursor, err := DecodeCursor(rawCursor)
		if err != nil {
			return q, BadRequest("cursor tidak valid")
		}
		q.After = &cursor
	}

	return q, nil
}
```

**`helper/validator.go`**:
```go
package helper

import (
	"errors"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New()

	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})

	_ = v.RegisterValidation("nospace", func(fl validator.FieldLevel) bool {
		return !strings.ContainsAny(fl.Field().String(), " \t\n\r")
	})

	_ = v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		for _, r := range fl.Field().String() {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
				r != '.' && r != '_' {
				return false
			}
		}
		return true
	})

	_ = v.RegisterValidation("strongpassword", func(fl validator.FieldLevel) bool {
		return passwordStrength(fl.Field().String()) == ""
	})

	return v
}

func ValidateStruct(s any) map[string]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}

	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return map[string]string{"_": "objek yang divalidasi tidak sah"}
	}

	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return map[string]string{"_": "validasi gagal"}
	}

	result := make(map[string]string, len(fieldErrors))
	for _, fe := range fieldErrors {
		if _, exists := result[fe.Field()]; !exists {
			result[fe.Field()] = messageFor(fe)
		}
	}
	return result
}

func messageFor(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "wajib diisi"
	case "email":
		return "format email tidak valid"
	case "min":
		if fe.Kind() == reflect.String {
			return "minimal " + fe.Param() + " karakter"
		}
		return "nilai minimal " + fe.Param()
	case "max":
		if fe.Kind() == reflect.String {
			return "maksimal " + fe.Param() + " karakter"
		}
		return "nilai maksimal " + fe.Param()
	case "gt":
		return "harus lebih besar dari " + fe.Param()
	case "gte":
		return "nilai minimal " + fe.Param()
	case "lt":
		return "harus lebih kecil dari " + fe.Param()
	case "lte":
		return "nilai maksimal " + fe.Param()
	case "alphanum":
		return "hanya boleh berisi huruf dan angka"
	case "nospace":
		return "tidak boleh mengandung spasi"
	case "username":
		return "hanya boleh huruf, angka, titik, dan garis bawah"
	case "strongpassword":
		if value, ok := fe.Value().(string); ok {
			return passwordStrength(value)
		}
		return "password tidak memenuhi syarat"
	case "oneof":
		return "harus salah satu dari: " + strings.ReplaceAll(fe.Param(), " ", ", ")
	default:
		return "tidak memenuhi aturan " + fe.Tag()
	}
}

func passwordStrength(password string) string {
	if len(password) < 8 {
		return "minimal 8 karakter"
	}

	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasDigit || !hasLetter {
		return "harus memuat huruf dan angka"
	}

	weak := map[string]bool{
		"password1": true, "12345678": true, "qwerty123": true,
		"admin123": true, "password123": true,
	}

	if weak[strings.ToLower(password)] {
		return "password terlalu umum"
	}

	return ""
}
```

**`helper/context.go`**:
```go
package helper

import (
	"courtmaster-backend/app/model"
	"github.com/gofiber/fiber/v2"
)

const LocalsAuthUser = "authUser"

func CurrentUser(c *fiber.Ctx) (model.AuthUser, bool) {
	user, ok := c.Locals(LocalsAuthUser).(model.AuthUser)
	return user, ok
}
```

**`helper/cursor.go`**:
```go
package helper

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"courtmaster-backend/app/model"
)

var ErrInvalidCursor = errors.New("cursor tidak valid")

func EncodeCursor(createdAt time.Time, id int64) string {
	raw := strconv.FormatInt(createdAt.UTC().UnixNano(), 10) + "|" + strconv.FormatInt(id, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(encoded string) (model.Cursor, error) {
	if strings.TrimSpace(encoded) == "" {
		return model.Cursor{}, ErrInvalidCursor
	}

	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return model.Cursor{}, ErrInvalidCursor
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return model.Cursor{}, ErrInvalidCursor
	}

	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return model.Cursor{}, ErrInvalidCursor
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id < 1 {
		return model.Cursor{}, ErrInvalidCursor
	}

	return model.Cursor{CreatedAt: time.Unix(0, nanos).UTC(), ID: id}, nil
}
```

```bash
git add helper/errors.go helper/response.go helper/request.go helper/validator.go helper/context.go helper/cursor.go
git commit -m "feat(helper): implement centralized error factory, response helpers, context extractor, and keyset cursor"
```

---

### Micro-Commit 6: Paket Helper Keamanan, Token, RBAC & Content Negotiation

**`helper/security.go`**:
```go
package helper

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var dummyHash = []byte("$2a$12$abcdefghijklmnopqrstuuLKa3Bt1TCmU/6zvhZ8x4nq1yBiuGvS")

func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

func VerifyDummyPassword(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(plain))
}

func RandomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
```

**`helper/jwt.go`**:
```go
package helper

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"courtmaster-backend/app/model"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("token tidak valid")
	ErrExpiredToken = errors.New("token sudah kedaluwarsa")
)

type UserClaims struct {
	User model.AuthUser `json:"user"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewJWTManager(secret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		issuer:     "courtmaster-backend",
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (m *JWTManager) GenerateAccessToken(u model.AuthUser) (string, error) {
	now := time.Now()
	claims := UserClaims{
		User: u,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(u.ID, 10),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) VerifyAccessToken(tokenStr string) (*UserClaims, error) {
	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("algoritma tidak diharapkan: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
```

**`helper/authz.go`**:
```go
package helper

import "sort"

// PermissionSet menyimpan pemetaan role ke daftar permission miliknya di memori (Modul 6).
type PermissionSet struct {
	byRole map[string]map[string]struct{}
}

// NewPermissionSet mengubah raw map dari database menjadi set yang cepat dicari (O(1)).
func NewPermissionSet(raw map[string][]string) *PermissionSet {
	byRole := make(map[string]map[string]struct{}, len(raw))

	for role, permissions := range raw {
		set := make(map[string]struct{}, len(permissions))
		for _, permission := range permissions {
			set[permission] = struct{}{}
		}
		byRole[role] = set
	}

	return &PermissionSet{byRole: byRole}
}

// Can memeriksa apakah sebuah role memiliki permission tertentu (prinsip fail closed).
func (p *PermissionSet) Can(role, permission string) bool {
	if p == nil {
		return false
	}

	permissions, ok := p.byRole[role]
	if !ok {
		return false
	}

	_, granted := permissions[permission]
	return granted
}

// PermissionsOf mengembalikan seluruh permission milik sebuah role, terurut.
func (p *PermissionSet) PermissionsOf(role string) []string {
	result := []string{}
	if p == nil {
		return result
	}

	for permission := range p.byRole[role] {
		result = append(result, permission)
	}
	sort.Strings(result)
	return result
}

// KnownRoles mengembalikan daftar role yang dikenal sistem, terurut.
func (p *PermissionSet) KnownRoles() []string {
	result := []string{}
	if p == nil {
		return result
	}

	for role := range p.byRole {
		result = append(result, role)
	}
	sort.Strings(result)
	return result
}

// IsKnownRole memeriksa apakah nama role terdaftar di sistem.
func (p *PermissionSet) IsKnownRole(role string) bool {
	if p == nil {
		return false
	}
	_, ok := p.byRole[role]
	return ok
}
```

**`helper/negotiate.go`**:
```go
package helper

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"courtmaster-backend/app/model"
	"github.com/gofiber/fiber/v2"
)

const (
	FormatJSON = fiber.MIMEApplicationJSON
	FormatCSV  = "text/csv"
)

func Negotiate(c *fiber.Ctx, offered ...string) (string, error) {
	accept := strings.TrimSpace(c.Get(fiber.HeaderAccept))
	if accept == "" {
		return offered[0], nil
	}
	chosen := c.Accepts(offered...)
	if chosen == "" {
		return "", NotAcceptable("format yang diminta tidak didukung, pilih: " + strings.Join(offered, ", "))
	}
	return chosen, nil
}

func WriteBookingsCSV(c *fiber.Ctx, bookings []model.Booking) error {
	c.Set(fiber.HeaderContentType, FormatCSV+"; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="bookings.csv"`)

	var buffer strings.Builder
	writer := csv.NewWriter(&buffer)

	header := []string{"id", "booking_code", "court_id", "date", "total_amount", "status", "created_at"}
	if err := writer.Write(header); err != nil {
		return Internal(err)
	}

	for _, b := range bookings {
		row := []string{
			strconv.FormatInt(b.ID, 10),
			b.BookingCode,
			strconv.FormatInt(b.CourtID, 10),
			b.Date,
			fmt.Sprintf("%.2f", b.TotalAmount),
			b.Status,
			b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if err := writer.Write(row); err != nil {
			return Internal(err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return Internal(err)
	}
	return c.SendString(buffer.String())
}
```

```bash
git add helper/security.go helper/jwt.go helper/authz.go helper/negotiate.go
git commit -m "feat(helper): implement security hashing, JWT manager, RBAC cache, and CSV negotiation"
```

---

### Micro-Commit 7: Skema DDL Fondasi RBAC & Pengguna (`001`, `002`) serta `RoleRepository`

Skema DDL fondasi awal disiapkan untuk sistem otorisasi dan identitas pengguna:

**`migrations/001_roles_and_permissions.sql`**:
```sql
-- roles & permissions (RBAC Modul 6)
CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) UNIQUE NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INT REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

INSERT INTO roles (id, name) VALUES 
    (1, 'admin'), 
    (2, 'manager'), 
    (3, 'player') 
ON CONFLICT DO NOTHING;

INSERT INTO permissions (name) VALUES
    ('court:create'), ('court:manage'),
    ('booking:create'), ('booking:read'), ('booking:update'), ('booking:export'),
    ('sparring:create'), ('sparring:join'),
    ('review:create'), ('review:moderate'),
    ('queue:monitor'), ('outbox:manage')
ON CONFLICT DO NOTHING;

-- Mapping permissions ke role
-- Admin: semua akses manajemen
INSERT INTO role_permissions (role_id, permission_id)
SELECT 1, id FROM permissions
ON CONFLICT DO NOTHING;

-- Manager: kelola lapangan & moderasi review
INSERT INTO role_permissions (role_id, permission_id)
SELECT 2, id FROM permissions WHERE name IN ('court:create', 'court:manage', 'review:moderate', 'booking:read')
ON CONFLICT DO NOTHING;

-- Player: booking, sparring, review
INSERT INTO role_permissions (role_id, permission_id)
SELECT 3, id FROM permissions WHERE name IN ('booking:create', 'booking:read', 'booking:update', 'booking:export', 'sparring:create', 'sparring:join', 'review:create')
ON CONFLICT DO NOTHING;
```

**`migrations/002_users.sql`**:
```sql
-- users: Inti identitas seluruh sistem (memuat username, email, dan daily_booking_hours sesuai SOP & Modul 10)
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role_id INT NOT NULL REFERENCES roles(id),
    daily_booking_hours INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed akun default (Password Bcrypt cost 12):
-- admin: AdminMaster2026
-- manager_central: ManagerPadel123
-- player_budi: BudiMainPadel2026
-- player_sari: SariBadminton2026
INSERT INTO users (username, name, email, password_hash, role_id) VALUES
    ('admin', 'Admin CourtMaster', 'admin@courtmaster.id', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 1),
    ('manager_central', 'Manager Central Arena', 'manager@courtmaster.id', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 2),
    ('player_budi', 'Budi Santoso', 'budi@gmail.com', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 3),
    ('player_sari', 'Sari Handayani', 'sari@gmail.com', '$2a$12$NqLqE6K4fO9G3wRzS5j46eQ3oVpU7eUfM5/wR6ZtP4d5W1jV.bQ8a', 3)
ON CONFLICT DO NOTHING;
```

**`app/repository/role_repository.go`** (Standar Modul 6 Langkah 3 & `api-students`):
```go
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RoleRepository interface {
	LoadPermissions(ctx context.Context) (map[string][]string, error)
}

type rolePostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) RoleRepository {
	return &rolePostgresRepository{pool: pool}
}

// LoadPermissions memuat seluruh pemetaan role ke permission sekali saat aplikasi booting
func (r *rolePostgresRepository) LoadPermissions(ctx context.Context) (map[string][]string, error) {
	query := `
		SELECT r.name, COALESCE(p.name, '')
		FROM roles r
		LEFT JOIN role_permissions rp ON rp.role_id = r.id
		LEFT JOIN permissions p ON p.id = rp.permission_id
		ORDER BY r.name, p.name
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("mengambil permission: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var role, perm string
		if err := rows.Scan(&role, &perm); err != nil {
			return nil, fmt.Errorf("membaca row permission: %w", err)
		}
		if _, ok := result[role]; !ok {
			result[role] = []string{}
		}
		if perm != "" {
			result[role] = append(result[role], perm)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("membaca hasil query permission: %w", err)
	}
	return result, nil
}
```

```bash
git add migrations/001_roles_and_permissions.sql migrations/002_users.sql app/repository/role_repository.go
git commit -m "feat(migration): initialize foundation schemas (roles, users) and RoleRepository for in-memory RBAC"
```

**Push Fondasi ke Remote (UNBLOCK TIM!):**
```bash
git push -u origin develop
```
> [!IMPORTANT]
> **Katakan di grup tim**: *"Base project sudah siap di branch `develop`. Helper sudah lengkap 100%, tabel users dan data seed awal sudah tersedia. Silakan git pull dan buat branch fitur masing-masing!"*

---

## 4. Rangkaian Kerja Anggota 1 — TAHAP 1 (Fitur Court di Branch `feat/court-management`)

Sekarang Anggota 1 membuat branch fitur khusus dan mengerjakan domain Court dengan micro-commit bertahap:

```bash
git checkout develop
git checkout -b feat/court-management
```

---

### Micro-Commit 8: Skema DDL Gelanggang & Model Domain Court

Buat skema DDL tabel lapangan dan slot jadwal di **`migrations/003_courts_and_slots.sql`**:
```sql
-- courts & slots (Domain Lapangan Modul 3, 4, 9)
CREATE TABLE IF NOT EXISTS courts (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    sport_type VARCHAR(30) NOT NULL,
    hourly_rate NUMERIC(12, 2) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS court_slots (
    id BIGSERIAL PRIMARY KEY,
    court_id BIGINT NOT NULL REFERENCES courts(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    booking_id BIGINT,
    version INT NOT NULL DEFAULT 1,
    CONSTRAINT uq_court_slot UNIQUE (court_id, date, start_time)
);

CREATE INDEX IF NOT EXISTS idx_slots_court_date ON court_slots(court_id, date);

-- Seed data lapangan & slot awal untuk testing instan
INSERT INTO courts (id, name, sport_type, hourly_rate, version, is_active) VALUES
    (1, 'Lapangan Padel A (Vinyl Pro)', 'padel', 150000.00, 1, true),
    (2, 'Lapangan Badminton 1 (Karpet Yonex)', 'badminton', 75000.00, 1, true)
ON CONFLICT (id) DO NOTHING;

-- Seed slot jam operasional hari ini & besok (08:00 - 12:00)
INSERT INTO court_slots (court_id, date, start_time, end_time, is_available) VALUES
    (1, CURRENT_DATE, '08:00:00', '09:00:00', true),
    (1, CURRENT_DATE, '09:00:00', '10:00:00', true),
    (1, CURRENT_DATE, '10:00:00', '11:00:00', true),
    (1, CURRENT_DATE, '11:00:00', '12:00:00', true),
    (1, CURRENT_DATE + INTERVAL '1 day', '08:00:00', '09:00:00', true),
    (1, CURRENT_DATE + INTERVAL '1 day', '09:00:00', '10:00:00', true),
    (2, CURRENT_DATE, '08:00:00', '09:00:00', true),
    (2, CURRENT_DATE, '09:00:00', '10:00:00', true)
ON CONFLICT DO NOTHING;
```

Buat file model entitas **`app/model/court.go`**:
```go
package model

import "time"

type Court struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	SportType  string    `json:"sport_type"`
	HourlyRate float64   `json:"hourly_rate"`
	Version    int       `json:"version"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

type CourtSlot struct {
	ID          int64     `json:"id"`
	CourtID     int64     `json:"court_id"`
	Date        string    `json:"date"`
	StartTime   string    `json:"start_time"`
	EndTime     string    `json:"end_time"`
	IsAvailable bool      `json:"is_available"`
	BookingID   *int64    `json:"booking_id,omitempty"`
	Version     int       `json:"version"`
}

type CreateCourtRequest struct {
	Name       string  `json:"name" validate:"required,min=3,max=100"`
	SportType  string  `json:"sport_type" validate:"required,oneof=padel badminton futsal basketball"`
	HourlyRate float64 `json:"hourly_rate" validate:"required,gt=0"`
}

type UpdateHourlyRateRequest struct {
	HourlyRate float64 `json:"hourly_rate" validate:"required,gt=0"`
	Version    int     `json:"version" validate:"required,gt=0"`
}
```

```bash
git add migrations/003_courts_and_slots.sql app/model/court.go
git commit -m "feat(court): create courts and court_slots schema with initial seed, and define Court entities"
```

---

### Micro-Commit 9: Repository Court & Optimistic Locking

**`app/repository/court_repository.go`**:
```go
package repository

import (
	"context"
	"errors"
	"fmt"

	"courtmaster-backend/app/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CourtRepository interface {
	WithTx(tx DBTX) CourtRepository
	Create(ctx context.Context, court *model.Court) error
	FindByID(ctx context.Context, id int64) (*model.Court, error)
	FindAll(ctx context.Context, sportType string) ([]model.Court, error)
	UpdateHourlyRateOptimistic(ctx context.Context, id int64, newRate float64, currentVersion int) error
	FindSlotsByCourtAndDate(ctx context.Context, courtID int64, date string) ([]model.CourtSlot, error)
}

type courtPostgresRepo struct {
	db DBTX
}

func NewCourtRepository(pool *pgxpool.Pool) CourtRepository {
	return &courtPostgresRepo{db: pool}
}

func (r *courtPostgresRepo) WithTx(tx DBTX) CourtRepository {
	return &courtPostgresRepo{db: tx}
}

func (r *courtPostgresRepo) Create(ctx context.Context, court *model.Court) error {
	query := `INSERT INTO courts (name, sport_type, hourly_rate, version, is_active)
	          VALUES ($1, $2, $3, 1, true) RETURNING id, created_at`
	err := r.db.QueryRow(ctx, query, court.Name, court.SportType, court.HourlyRate).Scan(&court.ID, &court.CreatedAt)
	if err != nil {
		return fmt.Errorf("courtRepo.Create: %w", err)
	}
	return nil
}

func (r *courtPostgresRepo) FindByID(ctx context.Context, id int64) (*model.Court, error) {
	query := `SELECT id, name, sport_type, hourly_rate, version, is_active, created_at FROM courts WHERE id = $1`
	var c model.Court
	err := r.db.QueryRow(ctx, query, id).Scan(&c.ID, &c.Name, &c.SportType, &c.HourlyRate, &c.Version, &c.IsActive, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *courtPostgresRepo) FindAll(ctx context.Context, sportType string) ([]model.Court, error) {
	query := `SELECT id, name, sport_type, hourly_rate, version, is_active, created_at FROM courts WHERE is_active = true`
	args := []any{}
	if sportType != "" {
		query += ` AND sport_type = $1`
		args = append(args, sportType)
	}
	query += ` ORDER BY id ASC`

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Court
	for rows.Next() {
		var c model.Court
		if err := rows.Scan(&c.ID, &c.Name, &c.SportType, &c.HourlyRate, &c.Version, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *courtPostgresRepo) UpdateHourlyRateOptimistic(ctx context.Context, id int64, newRate float64, currentVersion int) error {
	query := `UPDATE courts 
	          SET hourly_rate = $1, version = version + 1 
	          WHERE id = $2 AND version = $3`
	tag, err := r.db.Exec(ctx, query, newRate, id, currentVersion)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict // Versi data telah berubah di database
	}
	return nil
}

func (r *courtPostgresRepo) FindSlotsByCourtAndDate(ctx context.Context, courtID int64, date string) ([]model.CourtSlot, error) {
	query := `SELECT id, court_id, date::text, start_time::text, end_time::text, is_available, booking_id, version 
	          FROM court_slots WHERE court_id = $1 AND date = $2 ORDER BY start_time ASC`
	rows, err := r.db.Query(ctx, query, courtID, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.CourtSlot
	for rows.Next() {
		var s model.CourtSlot
		if err := rows.Scan(&s.ID, &s.CourtID, &s.Date, &s.StartTime, &s.EndTime, &s.IsAvailable, &s.BookingID, &s.Version); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}
```

```bash
git add app/repository/court_repository.go
git commit -m "feat(court): implement CourtRepository with optimistic locking"
```

---

### Micro-Commit 10: Pure Business Rules & Table-Driven Unit Tests

**`app/service/court_rules.go`**:
```go
package service

import (
	"errors"
	"strings"
)

var (
	ErrEmptyCourtName   = errors.New("nama lapangan tidak boleh kosong")
	ErrInvalidSportType = errors.New("jenis olahraga tidak valid, pilih: padel, badminton, futsal, atau basketball")
	ErrInvalidRate      = errors.New("tarif sewa per jam harus lebih besar dari 0")
	ErrMaxRateExceeded  = errors.New("tarif sewa per jam tidak boleh melebihi Rp 2.000.000")
)

var allowedSports = map[string]bool{
	"padel":      true,
	"badminton":  true,
	"futsal":     true,
	"basketball": true,
}

func ValidateCourtCreation(name string, sportType string, hourlyRate float64) error {
	if strings.TrimSpace(name) == "" {
		return ErrEmptyCourtName
	}
	sport := strings.ToLower(strings.TrimSpace(sportType))
	if !allowedSports[sport] {
		return ErrInvalidSportType
	}
	if hourlyRate <= 0 {
		return ErrInvalidRate
	}
	if hourlyRate > 2_000_000 {
		return ErrMaxRateExceeded
	}
	return nil
}
```

**`app/service/court_rules_test.go`**:
```go
package service

import (
	"errors"
	"testing"
)

func TestValidateCourtCreation(t *testing.T) {
	tests := []struct {
		name       string
		courtName  string
		sportType  string
		hourlyRate float64
		wantErr    error
	}{
		{"Valid Padel Court", "Lap A Padel", "padel", 150000, nil},
		{"Valid Badminton", "Court 1", "badminton", 60000, nil},
		{"Empty Name", "", "padel", 100000, ErrEmptyCourtName},
		{"Invalid Sport", "Lap 2", "tennis_meja", 50000, ErrInvalidSportType},
		{"Zero Rate", "Lap 3", "futsal", 0, ErrInvalidRate},
		{"Negative Rate", "Lap 4", "basketball", -5000, ErrInvalidRate},
		{"Exceeded Max Rate", "VIP Court", "padel", 2500000, ErrMaxRateExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCourtCreation(tt.courtName, tt.sportType, tt.hourlyRate)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected error: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}
```

```bash
git add app/service/court_rules.go app/service/court_rules_test.go
git commit -m "test(court): implement table-driven unit tests for court creation business rules"
```

---

### Micro-Commit 11: Fiber HTTP Handlers & Swagger Annotations

**`app/service/court_service.go`**:
```go
package service

import (
	"errors"
	"log/slog"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)

type CourtService struct {
	repo   repository.CourtRepository
	logger *slog.Logger
}

func NewCourtService(repo repository.CourtRepository, logger *slog.Logger) *CourtService {
	return &CourtService{repo: repo, logger: logger}
}

// ListCourts godoc
// @Summary Daftar seluruh gelanggang olahraga
// @Tags Courts
// @Produce json
// @Param sport_type query string false "Filter jenis olahraga"
// @Success 200 {object} model.WebResponse
// @Router /courts [get]
func (s *CourtService) ListCourts(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	sportType := c.Query("sport_type")
	courts, err := s.repo.FindAll(ctx, sportType)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar lapangan berhasil diambil", courts)
}

// GetCourtByID godoc
// @Summary Ambil detail lapangan berdasarkan ID
// @Tags Courts
// @Produce json
// @Param id path int true "Court ID"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id} [get]
func (s *CourtService) GetCourtByID(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	court, err := s.repo.FindByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.NotFound("lapangan tidak ditemukan")
	} else if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "detail lapangan berhasil diambil", court)
}

// ListSlots godoc
// @Summary Ambil jadwal slot jam ketersediaan lapangan
// @Tags Courts
// @Produce json
// @Param id path int true "Court ID"
// @Param date query string true "Tanggal format YYYY-MM-DD"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/slots [get]
func (s *CourtService) ListSlots(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}
	date := c.Query("date")
	if date == "" {
		return helper.BadRequest("parameter date wajib diisi (YYYY-MM-DD)")
	}

	slots, err := s.repo.FindSlotsByCourtAndDate(ctx, id, date)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "jadwal slot lapangan berhasil diambil", slots)
}

// CreateCourt godoc
// @Summary Tambah gelanggang baru (Admin/Manager)
// @Security BearerAuth
// @Tags Courts
// @Accept json
// @Produce json
// @Param request body model.CreateCourtRequest true "Data Lapangan"
// @Success 201 {object} model.WebResponse
// @Router /courts [post]
func (s *CourtService) CreateCourt(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.CreateCourtRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("format body JSON tidak valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidateCourtCreation(req.Name, req.SportType, req.HourlyRate); err != nil {
		return helper.BadRequest(err.Error())
	}

	court := model.Court{
		Name:       req.Name,
		SportType:  req.SportType,
		HourlyRate: req.HourlyRate,
	}
	if err := s.repo.Create(ctx, &court); err != nil {
		return helper.Internal(err)
	}
	return helper.Created(c, "lapangan berhasil ditambahkan", court, "")
}

// UpdateHourlyRate godoc
// @Summary Update tarif per jam dengan Optimistic Locking
// @Security BearerAuth
// @Tags Courts
// @Accept json
// @Produce json
// @Param id path int true "Court ID"
// @Param request body model.UpdateHourlyRateRequest true "Update Tarif"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/rate [patch]
func (s *CourtService) UpdateHourlyRate(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}
	var req model.UpdateHourlyRateRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("format body JSON tidak valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	err := s.repo.UpdateHourlyRateOptimistic(ctx, id, req.HourlyRate, req.Version)
	if errors.Is(err, repository.ErrConflict) {
		return helper.Conflict("gagal update tarif: data lapangan telah dimodifikasi oleh pengguna lain, silakan refresh")
	} else if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "tarif lapangan berhasil diupdate", nil)
}
```

```bash
git add app/service/court_service.go
git commit -m "feat(court): implement Fiber HTTP handlers and OpenAPI Swagger annotations"
```

**Push Branch Fitur Court & Buka PR #1:**

```bash
# Pastikan branch develop sudah ter-push terlebih dahulu ke remote origin
git push -u origin develop

# Push branch fitur ke remote origin
git push -u origin feat/court-management
```

#### Cara A: Menggunakan GitHub CLI (`gh`) di Terminal (Cepat & Otomatis)
```bash
gh pr create --base develop --head feat/court-management \
  --title "feat(court): implement court management and concurrency protection" \
  --body ""
```

#### Cara B: Melalui GitHub Web UI
1. Klik link pembuatan PR yang muncul otomatis pada log terminal setelah perintah `git push`, atau buka halaman repositori di web GitHub lalu klik tombol **Compare & pull request**.
2. Pastikan target branch: **base: `develop`** $\leftarrow$ **compare: `feat/court-management`**.
3. Isi **Title**:
   ```text
   feat(court): implement court management and concurrency protection
   ```
4. Kolom **Description** dapat dikosongkan, lalu klik **Create pull request**.

> [!IMPORTANT]
> **Merge PR #1 ke `develop` (Buka Akses untuk Anggota 3):**
> Sebagai Git Master, lakukan merge PR #1 (`feat/court-management`) ke branch `develop`:
> ```bash
> git checkout develop
> git merge --no-ff feat/court-management
> git push origin develop
> ```
> Beri tahu Anggota 3: *"PR #1 sudah dimerge ke `develop`. Silakan pull branch `develop` agar `repository.CourtRepository` dan model `Court` tersedia untuk BookingService Anda!"*

---

## 5. Rangkaian Kerja Anggota 1 — TAHAP 2 (Integrasi & Wiring Rute Opsi B)

Setelah **seluruh 5 Pull Request** disetujui dan di-merge ke branch `develop`:
- PR #1: `feat/court-management` (Anggota 1)
- PR #2: `feat/auth-rbac` (Anggota 2)
- PR #3: `feat/booking-reservation` (Anggota 3)
- PR #4: `feat/court-reviews` (Anggota 2)
- PR #5: `feat/outbox-worker` (Anggota 3)

Anggota 1 (selaku Git Master) berpindah ke branch `develop`, melakukan pull terbaru, dan memulai integrasi rute terpusat:

```bash
git checkout develop
git pull origin develop
```

---

### Micro-Commit 12: Konfigurasi Fiber App Instance & Centralized Error Handler

**`config/app.go`**:
```go
package config

import (
	"errors"
	"log/slog"

	"courtmaster-backend/app/model"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
)

func NewApp(logger *slog.Logger) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      GetEnv("APP_NAME", "CourtMaster Backend API"),
		ErrorHandler: newErrorHandler(logger),
		BodyLimit:    2 * 1024 * 1024,
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: GetEnv("ALLOWED_ORIGINS", "http://localhost:5173"),
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	if GetEnv("SWAGGER_ENABLED", "false") == "true" {
		app.Get("/swagger/*", swagger.HandlerDefault)
	}

	return app
}

func newErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		requestID, _ := c.Locals("requestid").(string)

		var appErr *helper.AppError

		switch {
		case errors.As(err, &appErr):
		case errors.Is(err, fiber.ErrRequestEntityTooLarge):
			appErr = &helper.AppError{
				Status:  fiber.StatusRequestEntityTooLarge,
				Code:    "PAYLOAD_TOO_LARGE",
				Message: "ukuran body melebihi batas yang diizinkan",
			}
		default:
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				appErr = &helper.AppError{
					Status:  fiberErr.Code,
					Code:    "HTTP_ERROR",
					Message: fiberErr.Message,
				}
			} else {
				appErr = helper.Internal(err)
			}
		}

		if appErr.Status >= fiber.StatusInternalServerError {
			logger.Error("request_failed",
				slog.String("request_id", requestID),
				slog.String("path", c.Path()),
				slog.String("code", appErr.Code),
				slog.Int("status", appErr.Status),
				slog.String("error", appErr.Error()),
			)
		} else {
			logger.Warn("request_rejected",
				slog.String("request_id", requestID),
				slog.String("path", c.Path()),
				slog.String("code", appErr.Code),
				slog.Int("status", appErr.Status),
			)
		}

		return c.Status(appErr.Status).JSON(model.ErrorResponse{
			Success:   false,
			Code:      appErr.Code,
			Message:   appErr.Message,
			Fields:    appErr.Fields,
			RequestID: requestID,
		})
	}
}
```

```bash
git add config/app.go
git commit -m "feat(app): configure Fiber app instance and centralized error handler"
```

---

### Micro-Commit 13: Wiring Rute Terpusat & Main Entry Point Graceful Shutdown

**`route/route.go`** (Satu File Tunggal Sesuai Standar Modul Perkuliahan):
```go
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

	// 1. Healthcheck
	api.Get("/health", healthCheck(deps.Pool, deps.Mongo))

	// 2. Auth Routes (Anggota 2)
	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login", middleware.LoginRateLimit(), deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout", middleware.RequireAuth(deps.JWT), deps.AuthService.Logout)
	auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)

	// 3. Court Routes (Anggota 1)
	courts := api.Group("/courts")
	courts.Get("/", deps.CourtService.ListCourts)
	courts.Get("/:id", deps.CourtService.GetCourtByID)
	courts.Get("/:id/slots", deps.CourtService.ListSlots)
	courts.Post("/", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "court:create"), deps.CourtService.CreateCourt)
	courts.Patch("/:id/rate", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "court:manage"), deps.CourtService.UpdateHourlyRate)
	courts.Patch("/:id", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "court:manage"), deps.CourtService.UpdateHourlyRate)

	// 4. Review Routes - MongoDB (Anggota 2)
	courts.Get("/:id/reviews", deps.ReviewService.ListReviews)
	courts.Get("/:id/reviews/summary", deps.ReviewService.GetCourtSummary)
	courts.Post("/:id/reviews", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "review:create"), deps.ReviewService.CreateReview)
	courts.Post("/:id/reviews/:reviewId/helpful", middleware.RequireAuth(deps.JWT), deps.ReviewService.VoteHelpful)
	courts.Post("/:id/reviews/:reviewId/replies", middleware.RequireAuth(deps.JWT), deps.ReviewService.AddReply)

	// Rute review langsung
	reviews := api.Group("/reviews", middleware.RequireAuth(deps.JWT))
	reviews.Post("/:id/helpful", deps.ReviewService.VoteHelpful)
	reviews.Post("/:id/replies", deps.ReviewService.AddReply)

	// 5. Booking Routes (Anggota 3)
	bookings := api.Group("/bookings", middleware.RequireAuth(deps.JWT))
	bookings.Post("/", middleware.RequirePermission(deps.Permissions, "booking:create"), deps.BookingService.CreateBooking)
	bookings.Get("/", middleware.RequirePermission(deps.Permissions, "booking:read"), deps.BookingService.ListUserBookings)
	bookings.Get("/my", middleware.RequirePermission(deps.Permissions, "booking:read"), deps.BookingService.ListUserBookings)
	bookings.Post("/swap", middleware.RequirePermission(deps.Permissions, "booking:update"), deps.BookingService.SwapBooking)
	bookings.Get("/export", middleware.RequirePermission(deps.Permissions, "booking:export"), deps.BookingService.ListUserBookings)

	// 6. Sparring Routes - Sesi Mabar (Anggota 3)
	sparrings := api.Group("/sparrings")
	sparrings.Get("/", deps.BookingService.ListSparrings)
	sparrings.Post("/", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "sparring:create"), deps.BookingService.CreateSparring)
	sparrings.Post("/:id/join", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "sparring:join"), deps.BookingService.JoinSparring)

	// 7. Notification & Queue Routes (Anggota 3)
	notifications := api.Group("/notifications", middleware.RequireAuth(deps.JWT))
	notifications.Get("/mine", deps.NotificationService.ListNotifications)
	notifications.Get("/", deps.NotificationService.ListNotifications)
	notifications.Post("/outbox/retry", middleware.RequirePermission(deps.Permissions, "outbox:manage"), deps.NotificationService.RetryOutbox)

	admin := api.Group("/admin", middleware.RequireAuth(deps.JWT), middleware.RequirePermission(deps.Permissions, "queue:monitor"))
	admin.Get("/queue", deps.NotificationService.GetQueueStats)

	// 8. Fallback 404 handler untuk route yang tidak terdaftar (Modul Perkuliahan & api-students)
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
```

**`main.go`**:
```go
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
	"courtmaster-backend/helper"
	"courtmaster-backend/route"
	"github.com/hibiken/asynq"
)

// @title CourtMaster Backend API
// @version 1.0
// @description Sistem Backend Reservasi Gelanggang Olahraga & Matchmaking Komunitas
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	role := flag.String("role", "all", "Role proses: api, worker, atau all")
	flag.Parse()

	config.LoadEnv()
	logger := config.NewLogger()

	if *role == "worker" {
		startWorker(logger)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 1. Inisialisasi Database
	pgPool, err := database.NewPool(ctx)
	if err != nil {
		logger.Error("failed_to_connect_postgres", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pgPool.Close()

	mongoClient, err := database.NewMongoClient(ctx)
	if err != nil {
		logger.Error("failed_to_connect_mongo", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() { _ = mongoClient.Disconnect(context.Background()) }()

	mongoDB := mongoClient.Database(config.GetEnv("MONGO_DATABASE", "courtmaster_mongo"))

	// 2. Wiring Repositories
	txManager := repository.NewTxManager(pgPool)
	courtRepo := repository.NewCourtRepository(pgPool)
	roleRepo := repository.NewRoleRepository(pgPool)
	userRepo := repository.NewUserRepository(pgPool)
	tokenRepo := repository.NewTokenRepository(pgPool)
	reviewRepo := repository.NewReviewRepository(mongoDB)
	bookingRepo := repository.NewBookingRepository(pgPool)
	outboxRepo := repository.NewOutboxRepository(pgPool)
	notifRepo := repository.NewNotificationRepository(pgPool)

	// 3. Redis & Asynq Clients
	redisOpt := asynq.RedisClientOpt{
		Addr:     config.GetEnv("REDIS_ADDR", "localhost:6379"),
		Password: config.GetEnv("REDIS_PASSWORD", ""),
		DB:       config.GetEnvInt("REDIS_DB", 0),
	}
	asynqClient := asynq.NewClient(redisOpt)
	defer asynqClient.Close()
	asynqInspector := asynq.NewInspector(redisOpt)
	defer asynqInspector.Close()

	// 4. JWT & In-Memory RBAC (Dimuat sekali dari Database via RoleRepository - Modul 6 Langkah 4)
	jwtManager := helper.NewJWTManager(
		config.GetEnv("JWT_SECRET", "supersecretkey_courtmaster_min32chars_demo!"),
		15*time.Minute,
		168*time.Hour,
	)

	rawPermissions, err := roleRepo.LoadPermissions(ctx)
	if err != nil {
		logger.Error("gagal_memuat_permissions_dari_db", slog.String("error", err.Error()))
		os.Exit(1)
	}
	permSet := helper.NewPermissionSet(rawPermissions)
	logger.Info("permission_dimuat", slog.Any("roles", permSet.KnownRoles()))

	// 5. Wiring Services Lengkap (Tidak ada yang bernilai nil)
	courtService := service.NewCourtService(courtRepo, logger)
	authService := service.NewAuthService(userRepo, tokenRepo, jwtManager, logger)
	reviewService := service.NewReviewService(reviewRepo, bookingRepo, logger)
	outboxRelay := service.NewOutboxRelay(outboxRepo, asynqClient, logger)
	bookingService := service.NewBookingService(bookingRepo, courtRepo, outboxRepo, txManager, logger)
	notificationService := service.NewNotificationService(notifRepo, outboxRepo, outboxRelay, asynqInspector, logger)

	// Jika role = all, jalankan background worker dalam goroutine terpisah
	if *role == "all" {
		go startWorker(logger)
	}

	// 6. Registrasi Router
	app := config.NewApp(logger)
	route.Register(app, route.Dependencies{
		Pool:                pgPool,
		Mongo:               mongoClient,
		JWT:                 jwtManager,
		Permissions:         permSet,
		CourtService:        courtService,
		AuthService:         authService,
		ReviewService:       reviewService,
		BookingService:      bookingService,
		NotificationService: notificationService,
	})

	// 7. Graceful Shutdown
	port := config.GetEnv("APP_PORT", "8080")
	go func() {
		logger.Info("server_running", slog.String("port", port))
		if err := app.Listen(":" + port); err != nil {
			logger.Info("server_stopped", slog.String("reason", err.Error()))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting_down_gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(shutdownCtx)
	logger.Info("server_exited_cleanly")
}

// Catatan: Fungsi startWorker(logger *slog.Logger) didefinisikan secara terpisah di file worker.go
// (dibuat oleh Anggota 3 pada PR #5 dalam package main).
// Saat kompilasi/run (go build . atau go run .), Go akan otomatis menggabungkan main.go dan worker.go.
```

```bash
git add route/route.go main.go
git commit -m "feat(route): register centralized API routes and main graceful shutdown runner with full service wiring"
```

---

### Micro-Commit 14: Multi-Stage Dockerfile & 5-Service Docker Compose

**`Dockerfile`**:
```dockerfile
# Stage 1: Build binary
FROM golang:alpine AS builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o courtmaster .

# Stage 2: Minimal runtime image
FROM scratch
WORKDIR /app
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/courtmaster /app/courtmaster
EXPOSE 8080
ENTRYPOINT ["/app/courtmaster"]
CMD ["-role=api"]
```

**`docker-compose.yml`**:
```yaml
version: '3.8'

x-app: &app
  build: .
  image: courtmaster:local
  restart: unless-stopped
  environment:
    APP_PORT: "8080"
    LOG_LEVEL: info
    DB_HOST: postgres
    DB_PORT: "5432"
    DB_USER: courtmaster
    DB_PASSWORD: ${DB_PASSWORD:-secret}
    DB_NAME: courtmaster_db
    DB_SSLMODE: disable
    DB_MAX_CONNS: "10"
    JWT_SECRET: ${JWT_SECRET:-supersecretkey_courtmaster_min32chars}
    MONGO_URI: mongodb://mongo:27017
    MONGO_DATABASE: courtmaster_mongo
    MONGO_MAX_POOL: "20"
    REDIS_ADDR: redis:6379
    REDIS_PASSWORD: ""
    REDIS_DB: "0"
    WORKER_CONCURRENCY: "5"
    RELAY_INTERVAL_MS: "2000"
    RELAY_BATCH: "50"
    ALLOWED_ORIGINS: "http://localhost:5173"
    SWAGGER_ENABLED: "true"
  depends_on:
    postgres:
      condition: service_healthy
    mongo:
      condition: service_healthy
    redis:
      condition: service_healthy

services:
  api:
    <<: *app
    command: ["-role=api"]
    ports:
      - "8080:8080"

  worker:
    <<: *app
    command: ["-role=worker"]

  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: courtmaster
      POSTGRES_PASSWORD: ${DB_PASSWORD:-secret}
      POSTGRES_DB: courtmaster_db
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U courtmaster -d courtmaster_db"]
      interval: 5s
      timeout: 5s
      retries: 5

  mongo:
    image: mongo:7.0
    restart: unless-stopped
    ports:
      - "27017:27017"
    volumes:
      - mongodata:/data/db
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    ports:
      - "6379:6379"
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
  mongodata:
  redisdata:
```

```bash
git add Dockerfile docker-compose.yml
git commit -m "chore(devops): configure multi-stage Dockerfile and full 5-service docker-compose"
git push origin develop
```

---

## 6. Ringkasan 14 Micro-Commits Anggota 1

| No | Commit Hash / Pesan | Scope / Kategori | Files yang Di-stage |
| :---: | :--- | :--- | :--- |
| **1** | `chore(setup): initialize go module...` | Setup Dependensi | `go.mod`, `go.sum`, `.gitignore`, `.env.example` |
| **2** | `chore(config): implement env & logger` | Konfigurasi | `config/env.go`, `config/logger.go` |
| **3** | `feat(database): pgxpool, mongo, tx & errors` | Database & ACID | `database/postgres.go`, `mongo.go`, `dbtx.go`, `tx.go`, `app/repository/errors.go` |
| **4** | `feat(model): WebResponse, AuthUser & Cursor` | Model Inti | `app/model/web_response.go`, `auth_user.go`, `cursor.go`, `booking.go` |
| **5** | `feat(helper): error factory, response, ctx & cursor` | Helper Inti | `helper/errors.go`, `response.go`, `request.go`, `validator.go`, `context.go`, `cursor.go` |
| **6** | `feat(helper): security, jwt, RBAC & CSV` | Helper Keamanan | `helper/security.go`, `jwt.go`, `authz.go`, `negotiate.go` |
| **7** | `feat(migration): foundation schemas & RoleRepo` | DDL & RBAC | `migrations/001_roles_and_permissions.sql`, `002_users.sql`, `app/repository/role_repository.go` |
| **8** | `feat(court): DDL 003 schema & Court entities` | DDL & Model | `migrations/003_courts_and_slots.sql`, `app/model/court.go` |
| **9** | `feat(court): CourtRepository` | Database Query | `app/repository/court_repository.go` |
| **10**| `test(court): business rules unit tests` | Unit Testing | `app/service/court_rules.go`, `court_rules_test.go` |
| **11**| `feat(court): Fiber handlers & Swagger` | HTTP Handlers | `app/service/court_service.go` |
| **12**| `feat(app): Fiber app instance & error` | Integrasi Server | `config/app.go` |
| **13**| `feat(route): register API routes & main` | Wiring Rute | `route/route.go`, `main.go` |
| **14**| `chore(devops): Dockerfile & compose` | Kontainerisasi | `Dockerfile`, `docker-compose.yml` |
