package middleware

import (
	"log/slog"
	"net/http"
	"net/url"

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
				return htmx.Redirect(c, resetPasswordURL(c.Request()))
			}
			return next(c)
		}
	}
}

// resetPasswordURL returns to the requested page after the reset, so an OAuth
// authorization request survives a forced password change.
func resetPasswordURL(request *http.Request) string {
	if request.Method != http.MethodGet || request.URL.RequestURI() == "/" {
		return "/reset-password"
	}
	query := url.Values{}
	query.Set("redirect", request.URL.RequestURI())
	return "/reset-password?" + query.Encode()
}
