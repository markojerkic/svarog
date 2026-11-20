package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archivelink"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func CustomHTTPErrorHandler(c *echo.Context, err error) {
	if resp, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && resp.Committed {
		return
	}

	code := http.StatusInternalServerError
	if status := echo.StatusCode(err); status != 0 {
		code = status
	}

	message := ""
	var he *echo.HTTPError
	if errors.As(err, &he) {
		message = he.Message
		if message == http.StatusText(code) {
			message = ""
		}
		slog.Debug("HTTP error", "code", code, "message", message)
	}

	path := c.Request().URL.Path
	if strings.HasPrefix(path, archivelink.Path) {
		path = archivelink.Path + ":token"
	}
	slog.Error("HTTP error", "code", code, "error", err, "path", path)

	props := pages.ErrorPageProps{
		StatusCode: code,
		Message:    message,
	}

	if err := utils.Render(c, code, pages.ErrorPage(props)); err != nil {
		slog.Error("Failed to render error page", "error", err)
		c.String(code, i18n.T(c.Request().Context(), "errors.500_title"))
	}
}
