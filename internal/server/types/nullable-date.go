package types

import (
	"fmt"
	"time"

	"github.com/labstack/echo/v5"
)

// NullableDate is a custom type for handling optional date inputs from forms.
// It implements echo.BindUnmarshaler to support multiple date formats.
type NullableDate struct {
	Time  time.Time
	Valid bool
}

var _ echo.BindUnmarshaler = (*NullableDate)(nil)

// UnmarshalParam implements echo's BindUnmarshaler interface for custom form binding.
// An empty value means no expiry. YYYY-MM-DD is a UTC calendar day and is stored as
// the exclusive end of that day, the start of the next UTC day, so the selected day
// remains valid through its end. RFC3339 values stay absolute instants.
func (nd *NullableDate) UnmarshalParam(param string) error {
	if param == "" {
		nd.Time = time.Time{}
		nd.Valid = false
		return nil
	}

	t, err := time.Parse("2006-01-02", param)
	if err == nil {
		nd.Time = t.AddDate(0, 0, 1)
		nd.Valid = true
		return nil
	}

	t, err = time.Parse(time.RFC3339, param)
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}

	nd.Time = t
	nd.Valid = true
	return nil
}
