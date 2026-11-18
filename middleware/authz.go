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
