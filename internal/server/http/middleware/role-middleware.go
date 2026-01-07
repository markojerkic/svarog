package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func RequiresRoleMiddleware(role auth.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
			if !ok || user == nil {
				htmx.AddErrorToast(c, "You are not logged in")
				return utils.HxRedirect(c, "/login")
			}

			if user.Role != role {
				htmx.AddErrorToast(c, "You don't have permission to access this page")
				return utils.HxRedirect(c, "/")
			}
			return next(c)
		}
	}
}
