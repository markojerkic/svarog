package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
)

func RequiresRoleMiddleware(role auth.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := c.Get("user").(*auth.LoggedInUser)
			if user == nil {
				return c.JSON(401, types.ApiError{Message: "Not logged in"})
			}

			if user.Role != role {
				return c.JSON(403, types.ApiError{Message: "Forbidden"})
			}
			return next(c)
		}
	}
}
