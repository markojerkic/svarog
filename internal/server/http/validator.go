package http

import (
	"context"
	"reflect"
	"strconv"
	"strings"

	"log/slog"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
)

type Validator struct {
	validator *validator.Validate
}

func NewValidator() *Validator {
	return &Validator{validator: validator.New()}
}

// Validate implements echo.Validator.
func (v *Validator) Validate(i interface{}) error {
	return v.ValidateContext(context.Background(), i)
}

func (v *Validator) ValidateContext(ctx context.Context, i any) error {
	if err := v.validator.Struct(i); err != nil {
		errs := err.(validator.ValidationErrors)
		slog.Error("Validation error", "error", errs)
		apiError := types.NewApiError(i18n.T(ctx, "validation.failed"))

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
			if jsonTag == "" {
				jsonTag = field.Tag.Get("form")
			}
			if jsonTag == "" {
				jsonTag = field.Name
			}

			// Split the json tag to handle cases like `json:"username,omitempty"`
			jsonName := strings.Split(jsonTag, ",")[0]

			// Use the json tag name instead of the struct field name
			apiError.Fields[jsonName] = validationMessage(ctx, e)
		}
		return apiError
	}
	return nil
}

func validationMessage(ctx context.Context, e validator.FieldError) string {
	limit := e.Param()
	switch e.Tag() {
	case "required":
		return i18n.T(ctx, "validation.required")
	case "gte":
		if isLengthCheck(e) {
			count, _ := strconv.Atoi(limit)
			return i18n.Format(ctx, i18n.Message{ID: "validation.min_length", Data: map[string]any{"Count": count}, Count: &count})
		}
		return i18n.Format(ctx, i18n.Message{ID: "validation.min_number", Data: map[string]any{"Limit": limit}})
	case "lte":
		if isLengthCheck(e) {
			count, _ := strconv.Atoi(limit)
			return i18n.Format(ctx, i18n.Message{ID: "validation.max_length", Data: map[string]any{"Count": count}, Count: &count})
		}
		return i18n.Format(ctx, i18n.Message{ID: "validation.max_number", Data: map[string]any{"Limit": limit}})
	case "oneof":
		options := strings.ReplaceAll(limit, " ", ", ")
		if e.Field() == "Role" && limit == "user admin" && i18n.LanguageFrom(ctx) == i18n.Croatian {
			options = i18n.T(ctx, "users.role_user") + ", " + i18n.T(ctx, "users.role_admin")
		}
		return i18n.Format(ctx, i18n.Message{ID: "validation.one_of", Data: map[string]any{"Options": options}})
	default:
		return i18n.T(ctx, "validation.invalid")
	}
}

func isLengthCheck(e validator.FieldError) bool {
	switch e.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return true
	default:
		return false
	}
}

var _ echo.Validator = &Validator{}
