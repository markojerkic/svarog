package middleware

import (
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
)

func RestPasswordMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*auth.LoggedInUser)
			if !ok || user == nil {
				log.Debug("No user in context, passing to next middleware")
				return next(c)
			}

			if user.NeedsPasswordReset && c.Path() != "/reset-password" {
				log.Debug("User needs password reset, redirecting to reset password page")
				htmx.ShowWarningToast(c, "Password reset required", "You need to reset your password to continue")
				return htmx.Redirect(c, "/reset-password")
			}
			return next(c)
		}
	}
}
