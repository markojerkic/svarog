package http

import (
	"reflect"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/types"
)

type Validator struct {
	validator *validator.Validate
}

// Validate implements echo.Validator.
func (v *Validator) Validate(i interface{}) error {
	if err := v.validator.Struct(i); err != nil {
		errs := err.(validator.ValidationErrors)
		log.Error("Validation error", "error", errs)
		apiError := types.NewApiError("Validation error")

		// Get the type of the struct, handling both pointer and non-pointer types
		t := reflect.TypeOf(i)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		for _, e := range errs {
			// Get the struct field
			field, _ := t.FieldByName(e.Field())

			// Get the json tag
			jsonTag := field.Tag.Get("json")
			// Split the json tag to handle cases like `json:"username,omitempty"`
			jsonName := strings.Split(jsonTag, ",")[0]

			// Use the json tag name instead of the struct field name
			apiError.Fields[jsonName] = e.Tag()
		}
		return apiError
	}
	return nil
}

var _ echo.Validator = &Validator{}
