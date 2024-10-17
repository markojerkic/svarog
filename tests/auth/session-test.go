package auth

import (
	"net/http"
	"net/http/httptest"

	authlayer "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestRegisterNewSession() {
	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	sessionStore := authlayer.NewMongoSessionStore(suite.mongoClient, []byte("markova-tajna"))
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)
}

func (suite *AuthSuite) TestGetSessionForNoUser() {
	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	sessionStore := authlayer.NewMongoSessionStore(suite.mongoClient, []byte("markova-tajna"))
	session, err := sessionStore.Get(req, "marko")
	assert.Error(t, err)
	assert.Nil(t, session)
}
func (suite *AuthSuite) TestSaveSession() {
	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	sessionStore := authlayer.NewMongoSessionStore(suite.mongoClient, []byte("markova-tajna"))
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)

	// Mock real behaviour
	suite.authService.Register(req.Context(), "marko", "marko")
	mockUser, err := suite.authService.GetUserByUsername(req.Context(), "marko")
	assert.NoError(t, err)

	session.Values["user_id"] = mockUser.ID.Hex()

	err = sessionStore.Save(req, httptest.NewRecorder(), session)
	assert.NoError(t, err)

	savedSession, err := sessionStore.Get(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, savedSession)
	assert.False(t, savedSession.IsNew)
}
