package types

import (
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
)

// NullableDate is a custom type for handling optional date inputs from forms.
// It implements echo.BindUnmarshaler to support multiple date formats.
type NullableDate struct {
	Time  time.Time
	Valid bool
}

var _ echo.BindUnmarshaler = (*NullableDate)(nil)

// UnmarshalParam implements echo's BindUnmarshaler interface for custom form binding.
// It supports both YYYY-MM-DD format (from datepicker) and RFC3339 format.
func (nd *NullableDate) UnmarshalParam(param string) error {
	if param == "" {
		nd.Valid = false
		return nil
	}

	// Try parsing as date only (YYYY-MM-DD) from datepicker
	t, err := time.Parse("2006-01-02", param)
	if err != nil {
		// Try RFC3339 format as fallback
		t, err = time.Parse(time.RFC3339, param)
		if err != nil {
			return fmt.Errorf("invalid date format: %w", err)
		}
	}

	nd.Time = t
	nd.Valid = true
	return nil
}
