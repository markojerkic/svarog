package types_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestFormBoolAcceptsCheckboxAndBooleanValues(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{"html checkbox default", "on", true, false},
		{"explicit true", "true", true, false},
		{"numeric true", "1", true, false},
		{"yes", " YES ", true, false},
		{"html off", "off", false, false},
		{"explicit false", "false", false, false},
		{"numeric false", "0", false, false},
		{"no", "no", false, false},
		{"empty is unchecked", "", false, false},
		{"unknown value", "maybe", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got types.FormBool
			err := got.UnmarshalParam(test.value)
			if test.wantErr {
				require.Error(t, err, "an unrecognized checkbox value must not be treated as false")
				return
			}
			require.NoError(t, err, "recognized checkbox and boolean values must bind")
			require.Equal(t, test.want, got.Bool(), "the bound value must match the submitted checkbox or boolean")
		})
	}
}

func TestFormBoolBindsThroughEcho(t *testing.T) {
	e := echo.New()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("enabled=on"))
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	var form struct {
		Enabled types.FormBool `form:"enabled"`
	}
	require.NoError(t, e.NewContext(request, httptest.NewRecorder()).Bind(&form), "Echo must use FormBool for the checkbox field")
	require.True(t, form.Enabled.Bool(), "a checked checkbox without an explicit value must bind as true")
}
