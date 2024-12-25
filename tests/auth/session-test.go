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

	// Create new session
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)

	// Create test user
	err = suite.authService.Register(req.Context(), "marko", "marko")
	assert.NoError(t, err)
	mockUser, err := suite.authService.GetUserByUsername(req.Context(), "marko")
	assert.NoError(t, err)
	assert.NotNil(t, mockUser)

	// Set user ID in session
	session.Values["user_id"] = mockUser.ID.Hex()

	// Save session and capture cookies
	responseRecorder := httptest.NewRecorder()
	err = sessionStore.Save(req, responseRecorder, session)
	assert.NoError(t, err)

	// Create new request with the session cookie
	cookies := responseRecorder.Result().Cookies()
	assert.NotEmpty(t, cookies, "Should have session cookie")

	newReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range cookies {
		newReq.AddCookie(cookie)
	}

	// Try to get the saved session
	savedSession, err := sessionStore.Get(newReq, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, savedSession)
	assert.False(t, savedSession.IsNew)

	// Verify user ID in retrieved session
	savedUserID, ok := savedSession.Values["user_id"].(string)
	assert.True(t, ok)
	assert.Equal(t, mockUser.ID.Hex(), savedUserID)
}
