package htmx

import (
	"encoding/json"
	"net/http"

	"github.com/a-h/templ"
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
)

func Redirect(c echo.Context, url string) error {
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", url)
		return c.NoContent(200)
	}
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func Render(c echo.Context, statusCode int, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(c.Request().Context(), buf); err != nil {
		return err
	}

	return c.HTML(statusCode, buf.String())
}

type ToastLevel string

const (
	LevelInfo    ToastLevel = "info"
	LevelSuccess ToastLevel = "success"
	LevelWarning ToastLevel = "warning"
	LevelError   ToastLevel = "error"
)

func AddToast(c echo.Context, message string, level ToastLevel) {
	var triggers map[string]any
	currentTrigger := c.Response().Header().Get("HX-Trigger")

	if currentTrigger != "" {
		if err := json.Unmarshal([]byte(currentTrigger), &triggers); err != nil {
			log.Error("Error parsing existing HX-Trigger header", "error", err)
			triggers = make(map[string]any)
		}
	} else {
		triggers = make(map[string]any)
	}

	triggers["toast"] = map[string]any{
		"message": message,
		"level":   string(level),
	}

	jsonData, err := json.Marshal(triggers)
	if err != nil {
		log.Error("Error encoding toast message", "error", err)
		return
	}

	c.Response().Header().Set("HX-Trigger", string(jsonData))
}

func AddSuccessToast(c echo.Context, message string) {
	AddToast(c, message, LevelSuccess)
}

func AddErrorToast(c echo.Context, message string) {
	AddToast(c, message, LevelError)
}

func AddInfoToast(c echo.Context, message string) {
	AddToast(c, message, LevelInfo)
}

func AddWarningToast(c echo.Context, message string) {
	AddToast(c, message, LevelWarning)
}
