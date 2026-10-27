package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
)

type FormBool bool

var _ echo.BindUnmarshaler = (*FormBool)(nil)

func (b *FormBool) UnmarshalParam(param string) error {
	value := strings.ToLower(strings.TrimSpace(param))
	switch value {
	case "", "off", "no":
		*b = false
		return nil
	case "on", "yes":
		*b = true
		return nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("invalid boolean %q", param)
	}
	*b = FormBool(parsed)
	return nil
}

func (b FormBool) Bool() bool {
	return bool(b)
}
