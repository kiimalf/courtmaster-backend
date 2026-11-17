# PANDUAN PENGERJAAN — ANGGOTA 2
## Peran: Security, RBAC, Rate Limiting & Community Reviews Lead
### Sistem: CourtMaster (Smart Sports Arena & Community Matchmaking)

---

## 1. Ikhtisar Tugas & Tanggung Jawab

Sebagai **Anggota 2**, Anda bertanggung jawab atas dua domain krusial yang dipecah menjadi **2 Pull Request (PR)** terpisah:
1. **PR 1 (`feat/auth-rbac`) — Autentikasi, JWT Refresh Token Rotation & RBAC (Modul 5, 6, 7)**:
   - DDL tabel `refresh_tokens`.
   - Proteksi registrasi & login dengan *Timing Attack Defense* (`helper.VerifyDummyPassword`).
   - Rotasi Refresh Token dengan pencatatan `revoked_at` dan hash SHA-256.
   - *Login Rate Limiter* (maksimal 5 percobaan gagal per 1 menit per IP/Email).
   - Middleware RBAC (*fail-closed permission checking*).
   - Terdiri dari **Micro-Commit 1 s.d. 6**.
2. **PR 2 (`feat/court-reviews`) — Sistem Ulasan Lapangan & Reaksi Komunitas NoSQL MongoDB (Modul 10)**:
   - CRUD Review lapangan di koleksi MongoDB `court_reviews`.
   - Update bersarang atomic dengan batas array (`$push` maksimal 50 balasan via slice `replies.49`).
   - Sistem voting *Helpful* idempotent (`$addToSet` dan `$pull`).
   - *Aggregation Pipeline* untuk menghitung rating rata-rata dan distribusi bintang (`$facet`, `$unwind`).
   - Terdiri dari **Micro-Commit 7 s.d. 10**.

---

## 2. Aturan Emas Pengerjaan Bebas Konflik (Wajib Dibaca!)

1. **Prinsip Foundation Freeze**:
   - Seluruh dependensi (`go.mod`), fungsi helper (`helper/`), driver database (`database/`), dan tabel `users` **sudah disediakan oleh Anggota 1 di branch `develop`**.
   - **DILARANG menjalankan `go get`**.
   - **DILARANG membuat atau mengedit file di folder `helper/`** (semua helper seperti `errors.go`, `jwt.go`, `security.go`, dll. sudah langsung aktif).
2. **Pola Integrasi Rute (Opsi B)**:
   - **DILARANG mengedit file `route/route.go` atau `main.go`** di branch Anda.
   - Uji logika bisnis Anda secara mandiri menggunakan **Unit Test (`auth_rules_test.go` & `review_rules_test.go`)**.
   - Cantumkan daftar endpoint di deskripsi Pull Request agar dihubungkan (*wired*) oleh Git Master (Anggota 1) saat integrasi di Tahap 2.

---

## 3. Eksekusi PR 1: `feat/auth-rbac` (Autentikasi, Token & Keamanan)

### Persiapan Branch PR 1:
```bash
git checkout develop
git pull origin develop
git checkout -b feat/auth-rbac
```

---

### Micro-Commit 1: DDL Migrasi Refresh Token

Tabel `users`, `roles`, dan `courts` sudah dibuat oleh Anggota 1 di migrasi `001`, `002`, dan `003`. Buat file **`migrations/004_refresh_tokens.sql`**:

```sql
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) UNIQUE NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens(user_id);
```

```bash
git add migrations/004_refresh_tokens.sql
git commit -m "feat(auth): create refresh_tokens table and indexes"
```

---

### Micro-Commit 2: Model Entitas & Request/Response DTOs Autentikasi

Buat file **`app/model/auth.go`**:

```go
package model

import "time"

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	RoleID       int       `json:"role_id"`
	RoleName     string    `json:"role_name,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type RefreshToken struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	TokenHash string     `json:"token_hash"`
	RevokedAt *time.Time `json:"revoked_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50,alphanum"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	RoleID   int    `json:"role_id" validate:"required,oneof=2 3"` // 2: Manager, 3: Player
}

type LoginRequest struct {
	Identifier string `json:"identifier"` // Email atau Username
	Email      string `json:"email"`      // Alternatif/backward compatibility
	Password   string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type AuthResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int64    `json:"expires_in"`
	User         AuthUser `json:"user"`
}
```

```bash
git add app/model/auth.go
git commit -m "feat(auth): define User, RefreshToken entities and auth request/response DTOs"
```

---

### Micro-Commit 3: Repository User & Token (PostgreSQL)

Buat file **`app/repository/user_repository.go`**:
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

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	FindByEmail(ctx context.Context, email string) (*model.User, error)
	FindByIdentifier(ctx context.Context, identifier string) (*model.User, error)
	FindByID(ctx context.Context, id int64) (*model.User, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]string, error)
}

type userPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userPostgresRepo{pool: pool}
}

func (r *userPostgresRepo) Create(ctx context.Context, user *model.User) error {
	query := `INSERT INTO users (username, name, email, password_hash, role_id)
	          VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, query, user.Username, user.Name, user.Email, user.PasswordHash, user.RoleID).
		Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return fmt.Errorf("userRepo.Create: %w", err)
	}
	return nil
}

func (r *userPostgresRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.FindByIdentifier(ctx, email)
}

func (r *userPostgresRepo) FindByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	query := `SELECT u.id, u.username, u.name, u.email, u.password_hash, u.role_id, r.name, u.created_at
	          FROM users u
	          JOIN roles r ON r.id = u.role_id
	          WHERE u.email = $1 OR u.username = $1`
	var u model.User
	err := r.pool.QueryRow(ctx, query, identifier).
		Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userPostgresRepo) FindByID(ctx context.Context, id int64) (*model.User, error) {
	query := `SELECT u.id, u.username, u.name, u.email, u.password_hash, u.role_id, r.name, u.created_at
	          FROM users u
	          JOIN roles r ON r.id = u.role_id
	          WHERE u.id = $1`
	var u model.User
	err := r.pool.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.RoleID, &u.RoleName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userPostgresRepo) GetUserPermissions(ctx context.Context, userID int64) ([]string, error) {
	query := `SELECT p.name
	          FROM users u
	          JOIN role_permissions rp ON rp.role_id = u.role_id
	          JOIN permissions p ON p.id = rp.permission_id
	          WHERE u.id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}
```

Buat file **`app/repository/token_repository.go`**:
```go
package repository

import (
	"context"
	"errors"
	"time"

	"courtmaster-backend/app/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRepository interface {
	Save(ctx context.Context, token *model.RefreshToken) error
	FindActiveByHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, id int64) error
	RevokeAllUserTokens(ctx context.Context, userID int64) error
}

type tokenPostgresRepo struct {
	pool *pgxpool.Pool
}

func NewTokenRepository(pool *pgxpool.Pool) TokenRepository {
	return &tokenPostgresRepo{pool: pool}
}

func (r *tokenPostgresRepo) Save(ctx context.Context, t *model.RefreshToken) error {
	query := `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
	          VALUES ($1, $2, $3) RETURNING id, created_at`
	return r.pool.QueryRow(ctx, query, t.UserID, t.TokenHash, t.ExpiresAt).Scan(&t.ID, &t.CreatedAt)
}

func (r *tokenPostgresRepo) FindActiveByHash(ctx context.Context, hash string) (*model.RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, revoked_at, expires_at, created_at
	          FROM refresh_tokens
	          WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`
	var t model.RefreshToken
	err := r.pool.QueryRow(ctx, query, hash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.RevokedAt, &t.ExpiresAt, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tokenPostgresRepo) Revoke(ctx context.Context, id int64) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, time.Now(), id)
	return err
}

func (r *tokenPostgresRepo) RevokeAllUserTokens(ctx context.Context, userID int64) error {
	query := `UPDATE refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, time.Now(), userID)
	return err
}
```

```bash
git add app/repository/user_repository.go app/repository/token_repository.go
git commit -m "feat(auth): implement UserRepository and TokenRepository with SHA256 token lookup"
```

---

### Micro-Commit 4: Pure Business Rules & Table-Driven Unit Tests

Buat file **`app/service/auth_rules.go`**:
```go
package service

import (
	"errors"
	"unicode"
)

var (
	ErrPasswordTooShort = errors.New("password minimal 8 karakter")
	ErrPasswordNoNumber = errors.New("password wajib memuat minimal 1 angka")
	ErrPasswordNoLetter = errors.New("password wajib memuat minimal 1 huruf")
	ErrSelfDemote       = errors.New("admin dilarang menurunkan role akunnya sendiri")
)

func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}
	var hasNumber, hasLetter bool
	for _, char := range password {
		if unicode.IsDigit(char) {
			hasNumber = true
		}
		if unicode.IsLetter(char) {
			hasLetter = true
		}
	}
	if !hasLetter {
		return ErrPasswordNoLetter
	}
	if !hasNumber {
		return ErrPasswordNoNumber
	}
	return nil
}

func ValidateRoleChange(currentUserID int64, targetUserID int64, currentRole string, targetRole string) error {
	if currentUserID == targetUserID && currentRole == "admin" && targetRole != "admin" {
		return ErrSelfDemote
	}
	return nil
}
```

Buat file **`app/service/auth_rules_test.go`**:
```go
package service

import (
	"errors"
	"testing"
)

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"Valid Password", "Rahasia123", nil},
		{"Too Short", "Pass1", ErrPasswordTooShort},
		{"No Number", "HanyaHurufSaja", ErrPasswordNoNumber},
		{"No Letter", "1234567890", ErrPasswordNoLetter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePasswordStrength(tt.password)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || err.Error() != tt.wantErr.Error()) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateRoleChange(t *testing.T) {
	// Admin tidak boleh menurunkan dirinya sendiri menjadi player
	err := ValidateRoleChange(1, 1, "admin", "player")
	if !errors.Is(err, ErrSelfDemote) {
		t.Fatalf("expected ErrSelfDemote, got: %v", err)
	}

	// Admin boleh mengubah role user lain
	err = ValidateRoleChange(1, 2, "admin", "player")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}
```

```bash
git add app/service/auth_rules.go app/service/auth_rules_test.go
git commit -m "test(auth): implement password rules, self-demote validation, and table-driven unit tests"
```

---

### Micro-Commit 5: Middleware Autentikasi, RBAC & Login Rate Limiter

Buat file **`middleware/auth.go`**:
```go
package middleware

import (
	"strings"

	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)

func RequireAuth(jwtManager *helper.JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get(fiber.HeaderAuthorization)
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return helper.Unauthorized("token autentikasi tidak ditemukan")
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := jwtManager.VerifyAccessToken(tokenStr)
		if err != nil {
			return helper.Unauthorized("token tidak valid atau sudah kedaluwarsa")
		}

		c.Locals(helper.LocalsAuthUser, claims.User)
		return c.Next()
	}
}

func RequireJSON(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodPost || c.Method() == fiber.MethodPut || c.Method() == fiber.MethodPatch {
		ct := c.Get(fiber.HeaderContentType)
		if !strings.HasPrefix(ct, fiber.MIMEApplicationJSON) {
			return helper.UnsupportedMediaType("Content-Type wajib application/json")
		}
	}
	return c.Next()
}
```

Buat file **`middleware/authz.go`**:
```go
package middleware

import (
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)

func RequirePermission(permSet *helper.PermissionSet, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := helper.CurrentUser(c)
		if !ok {
			return helper.Unauthorized("konteks pengguna tidak ditemukan")
		}

		if !permSet.Can(user.RoleName, permission) {
			return helper.Forbidden("anda tidak memiliki izin untuk tindakan ini")
		}
		return c.Next()
	}
}
```

Buat file **`middleware/ratelimit.go`**:
```go
package middleware

import (
	"time"

	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func LoginRateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			c.Set("Retry-After", "60")
			return helper.TooManyRequests("terlalu banyak percobaan login, coba lagi dalam satu menit")
		},
	})
}
```

```bash
git add middleware/auth.go middleware/authz.go middleware/ratelimit.go
git commit -m "feat(middleware): implement JWT validator, in-memory RBAC check, and IP login rate limiter"
```

---

### Micro-Commit 6: Fiber HTTP Handler AuthService & Timing Attack Defense

Buat file **`app/service/auth_service.go`**:
```go
package service

import (
	"errors"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)

type AuthService struct {
	userRepo   repository.UserRepository
	tokenRepo  repository.TokenRepository
	jwtManager *helper.JWTManager
	logger     *slog.Logger
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	jwtManager *helper.JWTManager,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

// Register godoc
// @Summary Pendaftaran akun pengguna baru
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.RegisterRequest true "Data Registrasi"
// @Success 201 {object} model.WebResponse
// @Router /auth/register [post]
func (s *AuthService) Register(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidatePasswordStrength(req.Password); err != nil {
		return helper.BadRequest(err.Error())
	}

	existing, _ := s.userRepo.FindByIdentifier(ctx, req.Email)
	if existing != nil {
		return helper.Conflict("email atau username sudah terdaftar")
	}
	existingUser, _ := s.userRepo.FindByIdentifier(ctx, req.Username)
	if existingUser != nil {
		return helper.Conflict("username sudah digunakan")
	}

	hash, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(err)
	}

	user := model.User{
		Username:     req.Username,
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: hash,
		RoleID:       req.RoleID,
	}
	if err := s.userRepo.Create(ctx, &user); err != nil {
		return helper.Internal(err)
	}

	return helper.Created(c, "registrasi berhasil", map[string]any{"user_id": user.ID, "username": user.Username, "email": user.Email}, "")
}

// Login godoc
// @Summary Login akun dan dapatkan Access Token + Refresh Token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.LoginRequest true "Kredensial Login"
// @Success 200 {object} model.WebResponse
// @Router /auth/login [post]
func (s *AuthService) Login(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	identifier := req.Email
	if req.Identifier != "" {
		identifier = req.Identifier
	}

	user, err := s.userRepo.FindByIdentifier(ctx, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		// Timing Attack Defense: tetap verifikasi dummy password agar durasi eksekusi konstan
		_ = helper.VerifyDummyPassword(req.Password)
		return helper.Unauthorized("email atau password salah")
	} else if err != nil {
		return helper.Internal(err)
	}

	if !helper.VerifyPassword(user.PasswordHash, req.Password) {
		return helper.Unauthorized("email atau password salah")
	}

	authUser := model.AuthUser{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.Name,
		RoleID:   user.RoleID,
		RoleName: user.RoleName,
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(authUser)
	if err != nil {
		return helper.Internal(err)
	}

	rawRefreshToken, err := helper.RandomToken(32)
	if err != nil {
		return helper.Internal(err)
	}

	tokenHash := helper.SHA256Hex(rawRefreshToken)
	rfToken := model.RefreshToken{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(168 * time.Hour), // 7 hari
	}
	if err := s.tokenRepo.Save(ctx, &rfToken); err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "login berhasil", model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
		User:         authUser,
	})
}

// Refresh godoc
// @Summary Rotasi Refresh Token untuk mendapatkan token baru
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body model.RefreshRequest true "Refresh Token"
// @Success 200 {object} model.WebResponse
// @Router /auth/refresh [post]
func (s *AuthService) Refresh(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	tokenHash := helper.SHA256Hex(req.RefreshToken)
	oldToken, err := s.tokenRepo.FindActiveByHash(ctx, tokenHash)
	if errors.Is(err, repository.ErrNotFound) {
		return helper.Unauthorized("refresh token tidak valid atau telah dicabut")
	} else if err != nil {
		return helper.Internal(err)
	}

	_ = s.tokenRepo.Revoke(ctx, oldToken.ID)

	user, err := s.userRepo.FindByID(ctx, oldToken.UserID)
	if err != nil {
		return helper.Unauthorized("pengguna tidak ditemukan")
	}

	authUser := model.AuthUser{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.Name,
		RoleID:   user.RoleID,
		RoleName: user.RoleName,
	}

	newAccessToken, err := s.jwtManager.GenerateAccessToken(authUser)
	if err != nil {
		return helper.Internal(err)
	}

	newRawRefresh, err := helper.RandomToken(32)
	if err != nil {
		return helper.Internal(err)
	}
	newHash := helper.SHA256Hex(newRawRefresh)

	newRf := model.RefreshToken{
		UserID:    user.ID,
		TokenHash: newHash,
		ExpiresAt: time.Now().Add(168 * time.Hour),
	}
	if err := s.tokenRepo.Save(ctx, &newRf); err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "token berhasil diperbarui", map[string]string{
		"access_token":  newAccessToken,
		"refresh_token": newRawRefresh,
	})
}

// Logout godoc
// @Summary Logout dan mencabut semua token aktif pengguna
// @Security BearerAuth
// @Tags Auth
// @Success 200 {object} model.WebResponse
// @Router /auth/logout [post]
func (s *AuthService) Logout(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	_ = s.tokenRepo.RevokeAllUserTokens(ctx, user.ID)
	return helper.Success(c, fiber.StatusOK, "logout berhasil, seluruh sesi dicabut", nil)
}

// Me godoc
// @Summary Profil pengguna yang sedang login
// @Security BearerAuth
// @Tags Auth
// @Success 200 {object} model.WebResponse
// @Router /auth/me [get]
func (s *AuthService) Me(c *fiber.Ctx) error {
	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	return helper.Success(c, fiber.StatusOK, "profil berhasil diambil", user)
}
```

```bash
git add app/service/auth_service.go
git commit -m "feat(auth): implement AuthService Fiber handlers with timing attack defense"
```

**Push Branch PR 1 & Buka PR #2:**
```bash
git push -u origin feat/auth-rbac
```

#### Cara A: Menggunakan GitHub CLI (`gh`) di Terminal
```bash
gh pr create --base develop --head feat/auth-rbac \
  --title "feat(auth): implement jwt authentication, refresh token rotation, and rbac" \
  --body ""
```

#### Cara B: Melalui GitHub Web UI
1. Klik link pembuatan PR yang muncul di log terminal setelah `git push`, atau buka repositori di web GitHub lalu klik **Compare & pull request**.
2. Pastikan target branch: **base: `develop`** $\leftarrow$ **compare: `feat/auth-rbac`**.
3. Isi **Title**:
   ```text
   feat(auth): implement jwt authentication, refresh token rotation, and rbac
   ```
4. Kolom **Description** dapat dikosongkan, lalu klik **Create pull request**.

> [!NOTE]
> **Prasyarat Memulai PR 2 (`feat/court-reviews`):**
> Pastikan PR #2 Anda (`feat/auth-rbac`) dan PR #3 milik Anggota 3 (`feat/booking-reservation`) sudah di-merge ke branch `develop` oleh Git Master.
> Dengan begitu, saat Anda menjalankan `git checkout develop && git pull origin develop`, interface `repository.BookingRepository` sudah tersedia di repositori lokal Anda untuk keperluan verifikasi riwayat sewa lapangan (`HasUserBookedCourt`).

---

## 4. Eksekusi PR 2: `feat/court-reviews` (Sistem Ulasan MongoDB)

### Persiapan Branch PR 2:
```bash
git checkout develop
git pull origin develop
git checkout -b feat/court-reviews
```

---

### Micro-Commit 7: Model Dokumen MongoDB BSON

Buat file **`app/model/review.go`**:
```go
package model

import "time"

type CourtReview struct {
	ID        string        `json:"id" bson:"_id,omitempty"`
	CourtID   int64         `json:"court_id" bson:"court_id"`
	UserID    int64         `json:"user_id" bson:"user_id"`
	UserName  string        `json:"user_name" bson:"user_name"`
	Rating    int           `json:"rating" bson:"rating"` // 1 s.d. 5
	Comment   string        `json:"comment" bson:"comment"`
	Tags      []string      `json:"tags" bson:"tags"`
	HelpfulBy []int64       `json:"helpful_by" bson:"helpful_by"`
	Replies   []ReviewReply `json:"replies" bson:"replies"`
	CreatedAt time.Time     `json:"created_at" bson:"created_at"`
}

type ReviewReply struct {
	UserID    int64     `json:"user_id" bson:"user_id"`
	UserName  string    `json:"user_name" bson:"user_name"`
	Comment   string    `json:"comment" bson:"comment"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}

type CreateReviewRequest struct {
	Rating  int      `json:"rating" validate:"required,min=1,max=5"`
	Comment string   `json:"comment" validate:"required,min=5,max=500"`
	Tags    []string `json:"tags" validate:"omitempty,max=5"`
}

type AddReplyRequest struct {
	Comment string `json:"comment" validate:"required,min=2,max=300"`
}

type ReviewSummary struct {
	AverageRating float64 `json:"average_rating"`
	TotalReviews  int     `json:"total_reviews"`
}
```

```bash
git add app/model/review.go
git commit -m "feat(review): define CourtReview BSON document, ReviewReply, and request DTOs"
```

---

### Micro-Commit 8: Repository MongoDB (Atomic Operators & Aggregation Pipeline)

Buat file **`app/repository/review_repository.go`**:
```go
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"courtmaster-backend/app/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ReviewRepository interface {
	Create(ctx context.Context, review *model.CourtReview) error
	FindByID(ctx context.Context, reviewID string) (*model.CourtReview, error)
	FindByCourtID(ctx context.Context, courtID int64) ([]model.CourtReview, error)
	AddReply(ctx context.Context, reviewID string, reply model.ReviewReply) error
	VoteHelpful(ctx context.Context, reviewID string, userID int64) error
	GetCourtSummary(ctx context.Context, courtID int64) (*model.ReviewSummary, error)
}

type reviewMongoRepo struct {
	col *mongo.Collection
}

type reviewDoc struct {
	ID        bson.ObjectID       `bson:"_id,omitempty"`
	CourtID   int64               `bson:"court_id"`
	UserID    int64               `bson:"user_id"`
	UserName  string              `bson:"user_name"`
	Rating    int                 `bson:"rating"`
	Comment   string              `bson:"comment"`
	Tags      []string            `bson:"tags"`
	HelpfulBy []int64             `bson:"helpful_by"`
	Replies   []model.ReviewReply `bson:"replies"`
	CreatedAt time.Time           `bson:"created_at"`
}

func NewReviewRepository(db *mongo.Database) ReviewRepository {
	col := db.Collection("court_reviews")
	return &reviewMongoRepo{col: col}
}

func (r *reviewMongoRepo) Create(ctx context.Context, rev *model.CourtReview) error {
	doc := reviewDoc{
		CourtID:   rev.CourtID,
		UserID:    rev.UserID,
		UserName:  rev.UserName,
		Rating:    rev.Rating,
		Comment:   rev.Comment,
		Tags:      rev.Tags,
		HelpfulBy: []int64{},
		Replies:   []model.ReviewReply{},
		CreatedAt: time.Now(),
	}

	res, err := r.col.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("reviewRepo.Create: %w", err)
	}
	if oid, ok := res.InsertedID.(bson.ObjectID); ok {
		rev.ID = oid.Hex()
		rev.CreatedAt = doc.CreatedAt
	}
	return nil
}

func (r *reviewMongoRepo) FindByID(ctx context.Context, reviewID string) (*model.CourtReview, error) {
	objID, err := bson.ObjectIDFromHex(reviewID)
	if err != nil {
		return nil, ErrNotFound
	}
	var doc reviewDoc
	err = r.col.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &model.CourtReview{
		ID:        doc.ID.Hex(),
		CourtID:   doc.CourtID,
		UserID:    doc.UserID,
		UserName:  doc.UserName,
		Rating:    doc.Rating,
		Comment:   doc.Comment,
		Tags:      doc.Tags,
		HelpfulBy: doc.HelpfulBy,
		Replies:   doc.Replies,
		CreatedAt: doc.CreatedAt,
	}, nil
}

func (r *reviewMongoRepo) FindByCourtID(ctx context.Context, courtID int64) ([]model.CourtReview, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{"court_id": courtID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []reviewDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	var reviews []model.CourtReview
	for _, d := range docs {
		reviews = append(reviews, model.CourtReview{
			ID:        d.ID.Hex(),
			CourtID:   d.CourtID,
			UserID:    d.UserID,
			UserName:  d.UserName,
			Rating:    d.Rating,
			Comment:   d.Comment,
			Tags:      d.Tags,
			HelpfulBy: d.HelpfulBy,
			Replies:   d.Replies,
			CreatedAt: d.CreatedAt,
		})
	}
	return reviews, nil
}

// Atomic update dengan $push berbatas maksimal 50 balasan
func (r *reviewMongoRepo) AddReply(ctx context.Context, reviewID string, reply model.ReviewReply) error {
	objID, err := bson.ObjectIDFromHex(reviewID)
	if err != nil {
		return err
	}
	update := bson.M{
		"$push": bson.M{
			"replies": bson.M{
				"$each":  []model.ReviewReply{reply},
				"$slice": -50,
			},
		},
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": objID}, update)
	return err
}

// Idempotent upvote menggunakan operator $addToSet
func (r *reviewMongoRepo) VoteHelpful(ctx context.Context, reviewID string, userID int64) error {
	objID, err := bson.ObjectIDFromHex(reviewID)
	if err != nil {
		return err
	}
	update := bson.M{
		"$addToSet": bson.M{"helpful_by": userID},
	}
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": objID}, update)
	return err
}

// Aggregation Pipeline menghitung rating rata-rata & total ulasan
func (r *reviewMongoRepo) GetCourtSummary(ctx context.Context, courtID int64) (*model.ReviewSummary, error) {
	pipeline := mongo.Pipeline{
		{{"$match", bson.M{"court_id": courtID}}},
		{{"$group", bson.M{
			"_id":           nil,
			"avg_rating":    bson.M{"$avg": "$rating"},
			"total_reviews": bson.M{"$sum": 1},
		}}},
	}

	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		AvgRating    float64 `bson:"avg_rating"`
		TotalReviews int     `bson:"total_reviews"`
	}
	if err := cursor.All(ctx, &results); err != nil || len(results) == 0 {
		return &model.ReviewSummary{AverageRating: 0, TotalReviews: 0}, nil
	}

	return &model.ReviewSummary{
		AverageRating: results[0].AvgRating,
		TotalReviews:  results[0].TotalReviews,
	}, nil
}
```

```bash
git add app/repository/review_repository.go
git commit -m "feat(review): implement MongoDB ReviewRepository with atomic push, addToSet, and aggregation pipeline"
```

---

### Micro-Commit 9: Pure Business Rules & Table-Driven Unit Tests Review

Buat file **`app/service/review_rules.go`**:
```go
package service

import (
	"errors"
	"strings"
)

var (
	ErrInvalidRating   = errors.New("rating harus berupa angka antara 1 sampai 5")
	ErrCommentEmpty    = errors.New("komentar minimal 5 karakter")
	ErrSpamDetected    = errors.New("komentar memuat konten terlarang atau spam")
	ErrReplyEmpty      = errors.New("balasan minimal 2 karakter")
	ErrSelfVoteBlocked = errors.New("pembuat ulasan dilarang memberikan upvote pada ulasannya sendiri")
)

var bannedKeywords = []string{"judi", "slot", "promo gratis", "wa.me"}

func ValidateReviewInput(rating int, comment string) error {
	if rating < 1 || rating > 5 {
		return ErrInvalidRating
	}
	trimmed := strings.TrimSpace(comment)
	if len(trimmed) < 5 {
		return ErrCommentEmpty
	}
	lower := strings.ToLower(trimmed)
	for _, kw := range bannedKeywords {
		if strings.Contains(lower, kw) {
			return ErrSpamDetected
		}
	}
	return nil
}

func ValidateReplyInput(comment string) error {
	trimmed := strings.TrimSpace(comment)
	if len(trimmed) < 2 {
		return ErrReplyEmpty
	}
	lower := strings.ToLower(trimmed)
	for _, kw := range bannedKeywords {
		if strings.Contains(lower, kw) {
			return ErrSpamDetected
		}
	}
	return nil
}

func ValidateHelpfulVote(authorUserID, voterUserID int64) error {
	if authorUserID == voterUserID {
		return ErrSelfVoteBlocked
	}
	return nil
}
```

Buat file **`app/service/review_rules_test.go`**:
```go
package service

import (
	"errors"
	"testing"
)

func TestValidateReviewInput(t *testing.T) {
	tests := []struct {
		name    string
		rating  int
		comment string
		wantErr error
	}{
		{"Valid Review", 5, "Lapangan sangat bagus dan bersih!", nil},
		{"Rating 0", 0, "Kurang memuaskan", ErrInvalidRating},
		{"Rating 6", 6, "Sangat luar biasa", ErrInvalidRating},
		{"Short Comment", 4, "ok", ErrCommentEmpty},
		{"Spam Content", 5, "Daftar slot online wa.me/123", ErrSpamDetected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateReviewInput(tt.rating, tt.comment)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
			if tt.wantErr != nil && (err == nil || !errors.Is(err, tt.wantErr)) {
				t.Fatalf("expected: %v, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateHelpfulVote(t *testing.T) {
	// Self-vote dilarang
	if err := ValidateHelpfulVote(10, 10); !errors.Is(err, ErrSelfVoteBlocked) {
		t.Fatalf("expected ErrSelfVoteBlocked, got: %v", err)
	}
	// Upvote dari user lain sah
	if err := ValidateHelpfulVote(10, 20); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}
```

```bash
git add app/service/review_rules.go app/service/review_rules_test.go
git commit -m "test(review): implement rating validation, spam filter, anti-self-vote rules, and unit tests"
```

---

### Micro-Commit 10: Fiber HTTP Handlers ReviewService & Anotasi Swagger

Buat file **`app/service/review_service.go`**:
```go
package service

import (
	"errors"
	"log/slog"
	"time"

	"courtmaster-backend/app/model"
	"courtmaster-backend/app/repository"
	"courtmaster-backend/helper"
	"github.com/gofiber/fiber/v2"
)

type ReviewService struct {
	repo        repository.ReviewRepository
	bookingRepo repository.BookingRepository
	logger      *slog.Logger
}

func NewReviewService(repo repository.ReviewRepository, bookingRepo repository.BookingRepository, logger *slog.Logger) *ReviewService {
	return &ReviewService{repo: repo, bookingRepo: bookingRepo, logger: logger}
}

// CreateReview godoc
// @Summary Beri ulasan dan rating pada lapangan (Wajib ada riwayat sewa di SQL)
// @Security BearerAuth
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path int true "Court ID"
// @Param request body model.CreateReviewRequest true "Data Review"
// @Success 201 {object} model.WebResponse
// @Router /courts/{id}/reviews [post]
func (s *ReviewService) CreateReview(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courtID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	var req model.CreateReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidateReviewInput(req.Rating, req.Comment); err != nil {
		return helper.BadRequest(err.Error())
	}

	// Verifikasi riwayat sewa di SQL sesuai SOP Bagian 3 & Modul 10
	if s.bookingRepo != nil {
		hasBooked, err := s.bookingRepo.HasUserBookedCourt(ctx, user.ID, courtID)
		if err != nil {
			return helper.Internal(err)
		}
		if !hasBooked {
			return helper.Forbidden("anda hanya dapat memberikan ulasan pada gelanggang yang pernah anda sewa")
		}
	}

	rev := model.CourtReview{
		CourtID:  courtID,
		UserID:   user.ID,
		UserName: user.FullName,
		Rating:   req.Rating,
		Comment:  req.Comment,
		Tags:     req.Tags,
	}

	if err := s.repo.Create(ctx, &rev); err != nil {
		return helper.Internal(err)
	}

	return helper.Created(c, "ulasan berhasil ditambahkan", rev, "")
}

// ListReviews godoc
// @Summary Ambil daftar ulasan gelanggang
// @Tags Reviews
// @Produce json
// @Param id path int true "Court ID"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/reviews [get]
func (s *ReviewService) ListReviews(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courtID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	reviews, err := s.repo.FindByCourtID(ctx, courtID)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "daftar ulasan berhasil diambil", reviews)
}

// GetCourtSummary godoc
// @Summary Ringkasan agregasi rating & total ulasan gelanggang
// @Tags Reviews
// @Produce json
// @Param id path int true "Court ID"
// @Success 200 {object} model.WebResponse
// @Router /courts/{id}/reviews/summary [get]
func (s *ReviewService) GetCourtSummary(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courtID, ok := helper.ParamID(c)
	if !ok {
		return helper.BadRequest("ID lapangan tidak valid")
	}

	summary, err := s.repo.GetCourtSummary(ctx, courtID)
	if err != nil {
		return helper.Internal(err)
	}

	return helper.Success(c, fiber.StatusOK, "ringkasan ulasan berhasil diambil", summary)
}

// VoteHelpful godoc
// @Summary Beri upvote bermanfaat pada ulasan (Anti-Self-Vote)
// @Security BearerAuth
// @Tags Reviews
// @Param id path string true "Review ID"
// @Success 200 {object} model.WebResponse
// @Router /reviews/{id}/helpful [post]
func (s *ReviewService) VoteHelpful(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	reviewID := c.Params("reviewId")
	if reviewID == "" {
		reviewID = c.Params("id")
	}

	rev, err := s.repo.FindByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return helper.NotFound("ulasan tidak ditemukan")
		}
		return helper.Internal(err)
	}

	if err := ValidateHelpfulVote(rev.UserID, user.ID); err != nil {
		return helper.BadRequest(err.Error())
	}

	if err := s.repo.VoteHelpful(ctx, reviewID, user.ID); err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "vote bermanfaat berhasil dicatat", nil)
}

// AddReply godoc
// @Summary Tambah balasan pada ulasan ($push atomik maksimal 50 balasan)
// @Security BearerAuth
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID"
// @Param request body model.AddReplyRequest true "Balasan Ulasan"
// @Success 200 {object} model.WebResponse
// @Router /reviews/{id}/replies [post]
func (s *ReviewService) AddReply(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	user, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	reviewID := c.Params("reviewId")
	if reviewID == "" {
		reviewID = c.Params("id")
	}

	var req model.AddReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body bukan JSON valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if err := ValidateReplyInput(req.Comment); err != nil {
		return helper.BadRequest(err.Error())
	}

	reply := model.ReviewReply{
		UserID:    user.ID,
		UserName:  user.FullName,
		Comment:   req.Comment,
		CreatedAt: time.Now(),
	}

	if err := s.repo.AddReply(ctx, reviewID, reply); err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "balasan berhasil ditambahkan", reply)
}
```

```bash
git add app/service/review_service.go
git commit -m "feat(review): implement ReviewService Fiber HTTP handlers with replies and anti-self-vote"
```

**Push Branch PR 2 & Buka PR #4:**
```bash
git push -u origin feat/court-reviews
```

#### Cara A: Menggunakan GitHub CLI (`gh`) di Terminal
```bash
gh pr create --base develop --head feat/court-reviews \
  --title "feat(review): implement mongodb court reviews, aggregation, and replies" \
  --body ""
```

#### Cara B: Melalui GitHub Web UI
1. Klik link otomatis yang muncul di terminal setelah `git push`, atau buka repositori di web GitHub lalu klik **Compare & pull request**.
2. Pastikan target branch: **base: `develop`** $\leftarrow$ **compare: `feat/court-reviews`**.
3. Isi **Title**:
   ```text
   feat(review): implement mongodb court reviews, aggregation, and replies
   ```
4. Kolom **Description** dapat dikosongkan, lalu klik **Create pull request**.

---

## 5. Ringkasan 10 Micro-Commits Anggota 2

| No | Commit Hash / Pesan | PR Target | Files yang Di-stage |
| :---: | :--- | :--- | :--- |
| **1** | `feat(auth): create refresh_tokens table...` | PR #2 (`feat/auth-rbac`) | `migrations/004_refresh_tokens.sql` |
| **2** | `feat(auth): define User, RefreshToken...` | PR #2 (`feat/auth-rbac`) | `app/model/auth.go` |
| **3** | `feat(auth): implement UserRepository...` | PR #2 (`feat/auth-rbac`) | `app/repository/user_repository.go`, `token_repository.go` |
| **4** | `test(auth): implement password rules...` | PR #2 (`feat/auth-rbac`) | `app/service/auth_rules.go`, `auth_rules_test.go` |
| **5** | `feat(middleware): JWT, RBAC, rate limiter` | PR #2 (`feat/auth-rbac`) | `middleware/auth.go`, `authz.go`, `ratelimit.go` |
| **6** | `feat(auth): AuthService Fiber handlers...` | PR #2 (`feat/auth-rbac`) | `app/service/auth_service.go` |
| **7** | `feat(review): define CourtReview BSON...` | PR #4 (`feat/court-reviews`) | `app/model/review.go` |
| **8** | `feat(review): MongoDB ReviewRepository...` | PR #4 (`feat/court-reviews`) | `app/repository/review_repository.go` |
| **9** | `test(review): rating rules & spam filter` | PR #4 (`feat/court-reviews`) | `app/service/review_rules.go`, `review_rules_test.go` |
| **10**| `feat(review): ReviewService handlers...` | PR #4 (`feat/court-reviews`) | `app/service/review_service.go` |
