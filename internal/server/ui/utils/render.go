package utils

import (
	"context"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v5"
)

type sidebarCollapsedKey struct{}

func SidebarCollapsed(ctx context.Context) bool {
	collapsed, _ := ctx.Value(sidebarCollapsedKey{}).(bool)
	return collapsed
}

func Render(ctx *echo.Context, statusCode int, t templ.Component) error {
	buf := templ.GetBuffer()
	defer templ.ReleaseBuffer(buf)

	renderCtx := ctx.Request().Context()
	if cookie, err := ctx.Cookie("sidebar_state"); err == nil && cookie.Value == "false" {
		renderCtx = context.WithValue(renderCtx, sidebarCollapsedKey{}, true)
	}
	if err := t.Render(renderCtx, buf); err != nil {
		return err
	}

	return ctx.HTML(statusCode, buf.String())
}

func HxRedirect(ctx *echo.Context, url string) error {
	if ctx.Request().Header.Get("HX-Request") == "true" {
		ctx.Response().Header().Set("HX-Location", url)
		return ctx.NoContent(200)
	}
	return ctx.Redirect(302, url)
}
