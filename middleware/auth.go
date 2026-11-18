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
