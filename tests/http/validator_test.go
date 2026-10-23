package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	serverhttp "github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
	"github.com/stretchr/testify/require"
)

func TestValidationMessagesDescribeTheRule(t *testing.T) {
	v := serverhttp.NewValidator()

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

func TestValidationUsesRequestLanguage(t *testing.T) {
	v := serverhttp.NewValidator()
	for _, test := range []struct {
		language string
		required string
		minimum  string
		maximum  string
		numeric  string
		role     string
	}{
		{"hr", "Ovo polje je obavezno.", "Unesite najmanje 1 znak.", "Unesite najviše 2 znaka.", "Vrijednost mora biti najmanje 1.", "Odaberite jednu od vrijednosti: Korisnik, Administrator."},
		{"en", "This field is required", "Must be at least 1 character", "Must be at most 2 characters", "Must be at least 1", "Must be one of: user, admin"},
	} {
		t.Run(test.language, func(t *testing.T) {
			t.Parallel()
			e := echo.New()
			e.Validator = v
			e.Use(middleware.LanguageMiddleware)
			e.POST("/", func(c *echo.Context) error {
				form := struct {
					Required string `json:"required" validate:"required"`
					Short    string `json:"short" validate:"gte=1"`
					Long     string `json:"long" validate:"lte=2"`
					Number   int    `json:"number" validate:"gte=1"`
					Role     string `json:"role" validate:"oneof=user admin"`
				}{Long: "abcd", Role: "invalid"}
				err := utils.Validate(c, &form)
				apiErr, ok := err.(types.ApiError)
				require.True(t, ok, "Context-aware validation must preserve field errors")
				return c.JSON(http.StatusBadRequest, apiErr)
			})
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Accept-Language", test.language)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			var apiErr types.ApiError
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &apiErr), "Validation must retain the existing JSON error contract")
			require.Equal(t, map[string]string{
				"required": test.required, "short": test.minimum, "long": test.maximum,
				"number": test.numeric, "role": test.role,
			}, apiErr.Fields, "Validation must use each request's language, plural rules and role labels without translating field identifiers")
		})
	}
}
