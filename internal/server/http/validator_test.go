package http

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestValidationMessagesDescribeTheRule(t *testing.T) {
	v := &Validator{validator: validator.New()}

	err := v.Validate(types.CreateUserForm{
		Username:  "qa-demo-check",
		FirstName: "Al",
		LastName:  "Demo",
		Role:      "nope",
	})
	apiErr, ok := err.(types.ApiError)
	require.True(t, ok, "Struct validation must return a field error the form can render")
	require.Equal(t, "Must be at least 3 characters", apiErr.Fields["firstName"], "A short name must state the minimum length, not the gte rule name")
	require.NotContains(t, apiErr.Fields["firstName"], "gte", "Internal validation rule names must not be shown as field messages")
	require.Equal(t, "Must be one of: user, admin", apiErr.Fields["role"], "Role errors must name the allowed values, not the oneof rule")

	err = v.Validate(types.GetProjectPageInput{Page: -1, Size: 101})
	apiErr, ok = err.(types.ApiError)
	require.True(t, ok, "Numeric validation must also return a field error")
	require.Equal(t, "Must be at least 0", apiErr.Fields["page"], "Numeric minimums must not be described as character lengths")
	require.Equal(t, "Must be at most 100", apiErr.Fields["size"], "Numeric maximums must state the limit, not the lte rule name")
}
