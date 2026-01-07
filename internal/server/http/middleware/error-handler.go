package middleware

import (
	"net/http"

	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	message := ""

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		if msg, ok := he.Message.(string); ok {
			slog.Debug("HTTP error", "code", code, "message", msg)
			message = msg
		}
		if apiErr, ok := he.Message.(types.ApiError); ok {
			slog.Debug("API error", "fields", apiErr.Fields)
			c.JSON(code, apiErr)
			return
		}
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
