package middleware

import (
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
)

func RequiresRoleMiddleware(role auth.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
			if !ok || user == nil {
				htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "auth.login_required"))
				return htmx.Redirect(c, "/login")
			}

			if user.Role != role {
				htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "auth.page_forbidden"))
				return htmx.Redirect(c, "/")
			}
			return next(c)
		}
	}
}
