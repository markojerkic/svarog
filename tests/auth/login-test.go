package auth

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func (suite *AuthSuite) TestLoginUsernamePass() {
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

	err = suite.authService.Login(ctx, "marko", "marko")
	assert.NoError(t, err)

	// Assert cookie and session
	cookie := rec.Result().Cookies()[0]
	assert.NotNil(t, cookie)
	assert.Equal(t, "svarog_session", cookie.Name)
}

func (suite *AuthSuite) TestLoginInvalidUsername() {
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

	err = suite.authService.Login(ctx, "marko1", "marko")
	assert.Error(t, err)
}

func (suite *AuthSuite) TestLoginWithToken() {
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

	var user auth.User
	err = suite.userCollection.FindOne(context.Background(), bson.M{"username": "marko"}).Decode(&user)
	assert.NoError(t, err)

	loginToken := user.LoginTokens[0]

	err = suite.authService.LoginWithToken(ctx, loginToken)
	assert.NoError(t, err)

	// Assert cookie and session
	cookie := rec.Result().Cookies()[0]
	assert.NotNil(t, cookie)
	assert.Equal(t, "svarog_session", cookie.Name)

	// Check that token is no longer in db
	err = suite.userCollection.FindOne(context.Background(), bson.M{"username": "marko"}).Decode(&user)
	assert.NoError(t, err)
	assert.Len(t, user.LoginTokens, 0)

}
