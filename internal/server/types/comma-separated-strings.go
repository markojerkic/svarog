package types

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// CommaSeparatedStrings is a custom type for handling comma-separated values from selectbox component.
// It implements echo.BindUnmarshaler to split comma-separated strings into a slice.
// This is needed because the templui selectbox component sends multiple values as "val1,val2,val3"
// instead of multiple form fields with the same name.
type CommaSeparatedStrings []string

var _ echo.BindUnmarshaler = (*CommaSeparatedStrings)(nil)

// UnmarshalParam implements echo's BindUnmarshaler interface for custom form binding.
// It splits comma-separated values into a string slice.
func (css *CommaSeparatedStrings) UnmarshalParam(param string) error {
	if param == "" {
		*css = []string{}
		return nil
	}

	// Split by comma and trim whitespace
	parts := strings.Split(param, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	*css = result
	return nil
}
