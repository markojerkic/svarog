package middleware

import (
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
)

func AuthContextMiddleware(authService auth.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := authService.GetCurrentUser(c)
			if err != nil {
				log.Debug("No user in context")
				return c.JSON(401, types.ApiError{Message: "Not logged in"})
			}
			c.Set("user", &user)
			return next(c)
		}
	}
}
