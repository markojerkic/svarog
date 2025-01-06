package middleware

import (
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
)

func AuthContextMiddleware(authService auth.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := authService.GetCurrentUser(c)
			if err != nil {
				log.Debug("Error getting current user", "error", err)
				return next(c)
			}
			c.Set("user", &user)
			return next(c)
		}
	}
}
