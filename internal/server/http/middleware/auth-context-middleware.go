package middleware

import (
	"context"
	"net/url"

	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
)

func AuthContextMiddleware(authService auth.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, err := authService.GetCurrentUser(c)
			if err != nil {
				slog.Debug("No user in context")
				returnUrl := c.Request().URL

				qp := url.Values{}
				qp.Set("redirect", returnUrl.Path+"?"+returnUrl.RawQuery)
				loginUrl := "/login?" + qp.Encode()
				slog.Debug("Redirecting to login", "returnUrl", returnUrl.String(), "loginUrl", loginUrl)

				return htmx.Redirect(c, loginUrl)
			}
			c.Set(auth.UserKey.String(), &user)
			c.Set(auth.IsAdminKey.String(), user.Role == auth.ADMIN)

			ctx := c.Request().Context()
			ctx = context.WithValue(ctx, auth.UserKey, &user)
			ctx = context.WithValue(ctx, auth.IsAdminKey, user.Role == auth.ADMIN)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
