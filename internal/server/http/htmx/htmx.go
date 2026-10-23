package htmx

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf16"

	"log/slog"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

func IsHXRequest(c *echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

func Redirect(c *echo.Context, url string) error {
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", url)
		return c.NoContent(200)
	}
	return c.Redirect(http.StatusTemporaryRedirect, url)
}

func Render(c *echo.Context, statusCode int, t templ.Component) error {
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

func AddToast(c *echo.Context, message string, level ToastLevel) {
	AddTrigger(c, "toast", map[string]any{
		"message": message,
		"level":   string(level),
	})
}

func AddTrigger(c *echo.Context, event string, detail any) {
	var triggers map[string]any
	currentTrigger := c.Response().Header().Get("HX-Trigger")

	if currentTrigger != "" {
		if err := json.Unmarshal([]byte(currentTrigger), &triggers); err != nil {
			slog.Error("Error parsing existing HX-Trigger header", "error", err)
			triggers = make(map[string]any)
		}
	} else {
		triggers = make(map[string]any)
	}

	triggers[event] = detail

	jsonData, err := json.Marshal(triggers)
	if err != nil {
		slog.Error("Error encoding toast message", "error", err)
		return
	}

	// Fetch decodes header bytes as Latin-1. Keep JSON ASCII-only so Croatian
	// text and supplementary Unicode characters survive the HTTP header intact.
	var header strings.Builder
	for _, r := range string(jsonData) {
		if r < 128 {
			header.WriteRune(r)
		} else if r <= 0xffff {
			fmt.Fprintf(&header, "\\u%04x", r)
		} else {
			high, low := utf16.EncodeRune(r)
			fmt.Fprintf(&header, "\\u%04x\\u%04x", high, low)
		}
	}
	c.Response().Header().Set("HX-Trigger", header.String())
}

func AddSuccessToast(c *echo.Context, message string) {
	AddToast(c, message, LevelSuccess)
}

func AddErrorToast(c *echo.Context, message string) {
	AddToast(c, message, LevelError)
}

func AddInfoToast(c *echo.Context, message string) {
	AddToast(c, message, LevelInfo)
}

func AddWarningToast(c *echo.Context, message string) {
	AddToast(c, message, LevelWarning)
}

func HxVals(vals map[string]string) string {
	val, err := templ.JSONString(vals)
	if err != nil {
		slog.Error("Error encoding vals", "error", err)
		return ""
	}
	return val
}
