package middleware

import (
	"log/slog"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

func RestPasswordMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user, ok := c.Get("user").(*auth.LoggedInUser)
			if !ok || user == nil {
				slog.Debug("No user in context, passing to next middleware")
				return next(c)
			}

			if user.NeedsPasswordReset && c.Path() != "/reset-password" {
				slog.Debug("User needs password reset, redirecting to reset password page")
				htmx.AddWarningToast(c, i18n.T(c.Request().Context(), "auth.reset_required"))
				return htmx.Redirect(c, "/reset-password")
			}
			return next(c)
		}
	}
}
