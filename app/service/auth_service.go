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
		helper.VerifyDummyPassword(req.Password)
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
