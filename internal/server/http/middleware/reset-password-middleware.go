package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"log/slog"
)

func RestPasswordMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*auth.LoggedInUser)
			if !ok || user == nil {
				slog.Debug("No user in context, passing to next middleware")
				return next(c)
			}

			if user.NeedsPasswordReset && c.Path() != "/reset-password" {
				slog.Debug("User needs password reset, redirecting to reset password page")
				htmx.AddWarningToast(c, "You need to reset your password to continue")
				return htmx.Redirect(c, "/reset-password")
			}
			return next(c)
		}
	}
}
