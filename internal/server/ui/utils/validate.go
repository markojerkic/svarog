package utils

import (
	"context"

	"github.com/labstack/echo/v5"
)

// Validate passes the request language to validators that support context.
func Validate(c *echo.Context, input any) error {
	if validator, ok := c.Echo().Validator.(interface {
		ValidateContext(context.Context, any) error
	}); ok {
		return validator.ValidateContext(c.Request().Context(), input)
	}
	return c.Validate(input)
}
