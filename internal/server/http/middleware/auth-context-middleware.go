package middleware

import (
	"net/http"

	"log/slog"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
)

func AuthContextMiddleware(authService auth.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := authService.GetCurrentUser(c)
			if err != nil {
				slog.Debug("No user in context")
				return c.Redirect(http.StatusTemporaryRedirect, "/login")
			}
			c.Set("user", &user)
			return next(c)
		}
	}
}
