package auth

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestResetPassword() {
	t := suite.T()

	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	_, err := suite.authService.Register(ctx, types.RegisterForm{
		Username:  "marko",
		Password:  "marko",
		FirstName: "Marko",
		LastName:  "Jerkic",
	})
	assert.NoError(t, err)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)

	err = suite.authService.Login(ctx, "marko", "marko")
	assert.NoError(t, err)

	sessionCookie := rec.Result().Cookies()[0]

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	ctx = e.NewContext(req, rec)

	req.AddCookie(sessionCookie)

	err = suite.authService.ResetPassword(ctx, types.ResetPasswordForm{
		Password:         "marko1",
		RepeatedPassword: "marko1",
	})
	assert.NoError(t, err)

	err = suite.authService.Login(ctx, "marko", "marko1")
	assert.NoError(t, err)

	user, err := suite.authService.GetUserByUsername(context.Background(), "marko")
	assert.NoError(t, err)

	assert.False(t, user.NeedsPasswordReset)
}
