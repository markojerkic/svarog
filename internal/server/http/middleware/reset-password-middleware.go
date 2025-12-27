package middleware

import (
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
)

func RestPasswordMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*auth.LoggedInUser)
			if !ok || user == nil {
				log.Debug("No user in context, passing to next middleware")
				return next(c)
			}

			if user.NeedsPasswordReset && c.Path() != "/api/v1/auth/reset-password" && c.Path() != "/auth/reset-password" {
				return c.JSON(401, types.ApiError{Message: "password_reset_required"})
			}
			return next(c)
		}
	}
}
