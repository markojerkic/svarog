package middleware

import (
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
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
			message = msg
		}
	}

	log.Error("HTTP error", "code", code, "error", err, "path", c.Request().URL.Path)

	props := pages.ErrorPageProps{
		StatusCode: code,
		Message:    message,
	}

	if err := utils.Render(c, code, pages.ErrorPage(props)); err != nil {
		log.Error("Failed to render error page", "error", err)
		c.String(code, "Internal Server Error")
	}
}
