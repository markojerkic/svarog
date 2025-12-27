package utils

import (
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

func Render(ctx echo.Context, statusCode int, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	if err := t.Render(ctx.Request().Context(), buf); err != nil {
		return err
	}

	return ctx.HTML(statusCode, buf.String())
}

func HxRedirect(ctx echo.Context, url string) error {
	if ctx.Request().Header.Get("HX-Request") == "true" {
		ctx.Response().Header().Set("HX-Location", url)
		return ctx.NoContent(200)
	}
	return ctx.Redirect(302, url)
}
