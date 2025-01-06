package auth

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/charmbracelet/log"
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

	_, err := suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
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

	log.Info("User saved", "user", savedUser)
}

func (suite *AuthSuite) TestRegisterExistingUser() {
	t := suite.T()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	_, err := suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
		FirstName: "Marko",
		LastName:  "Jerkic",
	})
	assert.NoError(t, err)

	_, err = suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
		FirstName: "Marko",
		LastName:  "Jerkic",
	})
	assert.Error(t, err)
}
