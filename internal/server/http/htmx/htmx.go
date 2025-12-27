package htmx

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/ui/components/toast"
)

func Redirect(c echo.Context, url string) error {
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", url)
		log.Debug("Redirecting with htmx", "url", url)
		return c.NoContent(200)
	}
	log.Debug("Redirecting without htmx", "url", url)
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

type ToastOptions struct {
	Title       string
	Description string
	Level       ToastLevel
	Duration    int
	Dismissible bool
}

func ShowToast(c echo.Context, opts ToastOptions) error {
	variant := toast.VariantDefault
	switch opts.Level {
	case LevelInfo:
		variant = toast.VariantInfo
	case LevelSuccess:
		variant = toast.VariantSuccess
	case LevelWarning:
		variant = toast.VariantWarning
	case LevelError:
		variant = toast.VariantError
	}

	duration := opts.Duration
	if duration == 0 {
		duration = 3000
	}

	return Render(c, 200, toast.Toast(toast.Props{
		Title:         opts.Title,
		Description:   opts.Description,
		Variant:       variant,
		Duration:      duration,
		Dismissible:   opts.Dismissible,
		Icon:          true,
		ShowIndicator: true,
	}))
}

func ShowSuccessToast(c echo.Context, title string, description string) error {
	return ShowToast(c, ToastOptions{
		Title:       title,
		Description: description,
		Level:       LevelSuccess,
		Dismissible: true,
	})
}

func ShowErrorToast(c echo.Context, title string, description string) error {
	return ShowToast(c, ToastOptions{
		Title:       title,
		Description: description,
		Level:       LevelError,
		Dismissible: true,
	})
}

func ShowInfoToast(c echo.Context, title string, description string) error {
	return ShowToast(c, ToastOptions{
		Title:       title,
		Description: description,
		Level:       LevelInfo,
		Dismissible: true,
	})
}

func ShowWarningToast(c echo.Context, title string, description string) error {
	return ShowToast(c, ToastOptions{
		Title:       title,
		Description: description,
		Level:       LevelWarning,
		Dismissible: true,
	})
}

func RenderWithToast(c echo.Context, statusCode int, t templ.Component, opts ToastOptions) error {
	variant := toast.VariantDefault
	switch opts.Level {
	case LevelInfo:
		variant = toast.VariantInfo
	case LevelSuccess:
		variant = toast.VariantSuccess
	case LevelWarning:
		variant = toast.VariantWarning
	case LevelError:
		variant = toast.VariantError
	}

	duration := opts.Duration
	if duration == 0 {
		duration = 3000
	}

	toastComponent := toast.Toast(toast.Props{
		Title:         opts.Title,
		Description:   opts.Description,
		Variant:       variant,
		Duration:      duration,
		Dismissible:   opts.Dismissible,
		Icon:          true,
		ShowIndicator: true,
	})

	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(c.Request().Context(), buf); err != nil {
		return err
	}
	if err := toastComponent.Render(c.Request().Context(), buf); err != nil {
		return err
	}

	return c.HTML(statusCode, buf.String())
}
