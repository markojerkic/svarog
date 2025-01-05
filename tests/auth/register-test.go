package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	authlayer "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestRegisterNewUser() {
	t := suite.T()

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
		Password:  "marko",
		FirstName: "Marko",
		LastName:  "Jerkic",
	})
	assert.NoError(t, err)

	savedUser, err := suite.authService.GetUserByUsername(context.Background(), "marko")

	assert.NoError(t, err)
	assert.NotNil(t, savedUser)
	assert.Equal(t, "marko", savedUser.Username)
	assert.Equal(t, authlayer.USER, savedUser.Role)
	assert.NotEmpty(t, savedUser.Password)
	assert.NotEqual(t, "marko", savedUser.Password)

	slog.Info("User saved", slog.Any("user", savedUser))
}

func (suite *AuthSuite) TestRegisterExistingUser() {
	t := suite.T()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
		Password:  "marko",
		FirstName: "Marko",
		LastName:  "Jerkic",
	})
	assert.NoError(t, err)

	err = suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
		Password:  "marko",
		FirstName: "Marko",
		LastName:  "Jerkic",
	})
	assert.Error(t, err)
}
