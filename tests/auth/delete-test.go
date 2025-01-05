package auth

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func (suite *AuthSuite) TestDeleteUser() {
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

	user, err := suite.authService.GetUserByUsername(context.Background(), "marko")
	assert.NoError(t, err)

	err = suite.authService.Login(ctx, "marko", "marko")
	assert.NoError(t, err)

	err = suite.authService.DeleteUser(ctx, "marko")
	assert.NoError(t, err)

	// Assert no user or sessions in db
	foundUser := suite.userCollection.FindOne(context.Background(), bson.M{"username": "marko"})
	assert.Error(t, foundUser.Err())

	sessions, err := suite.sessionCollection.CountDocuments(context.Background(), bson.M{"user_id": user.ID})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), sessions)

}
