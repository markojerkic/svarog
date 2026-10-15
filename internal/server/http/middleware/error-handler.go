package middleware

import (
	"errors"
	"net/http"

	"log/slog"

	"github.com/labstack/echo/v5"
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
		slog.Debug("HTTP error", "code", code, "message", message)
	}

	slog.Error("HTTP error", "code", code, "error", err, "path", c.Request().URL.Path)

	props := pages.ErrorPageProps{
		StatusCode: code,
		Message:    message,
	}

	if err := utils.Render(c, code, pages.ErrorPage(props)); err != nil {
		slog.Error("Failed to render error page", "error", err)
		c.String(code, "Internal Server Error")
	}
}
