package auth

import (
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestRegisterNewSession() {
	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	sessionStore := suite.sessionStore
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)
}

func (suite *AuthSuite) TestGetSessionForNoUser() {
	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	sessionStore := suite.sessionStore
	session, err := sessionStore.Get(req, "marko")
	assert.Error(t, err)
	assert.Nil(t, session)
}

func (suite *AuthSuite) TestSaveSession() {
	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	sessionStore := suite.sessionStore

	// Create new session
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)

	e := echo.New()
	ctx := e.NewContext(req, rec)

	// Create test user
	err = suite.authService.Register(ctx, "marko", "marko")
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

func TestSessionExpiration(suite *AuthSuite) {
	// Set up session with old modified time
	// Verify it's considered expired

	t := suite.T()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	sessionStore := suite.sessionStore

	e := echo.New()
	ctx := e.NewContext(req, rec)

	// Create new session
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)

	// Create test user
	err = suite.authService.Register(ctx, "marko", "marko")
	assert.NoError(t, err)
	mockUser, err := suite.authService.GetUserByUsername(req.Context(), "marko")
	assert.NoError(t, err)
	assert.NotNil(t, mockUser)

	// Set user ID in session
	session.Values["user_id"] = mockUser.ID.Hex()

	// Set old modified time
	session.Options.MaxAge = -1

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
	assert.Error(t, err)
	assert.Nil(t, savedSession)

}

func TestConcurrentSessions(suite *AuthSuite) {
	// Create multiple sessions for same user
	// Verify they don't interfere
	t := suite.T()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	e := echo.New()
	ctx := e.NewContext(req, rec)

	sessionStore := suite.sessionStore

	// Create new session
	session, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session)
	assert.True(t, session.IsNew)

	// Create test user
	err = suite.authService.Register(ctx, "marko", "marko")
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

	// Create another session for the same user
	session2, err := sessionStore.New(req, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, session2)
	assert.True(t, session2.IsNew)

	// Set user ID in session
	session2.Values["user_id"] = mockUser.ID.Hex()

	// Save session and capture cookies
	responseRecorder2 := httptest.NewRecorder()
	err = sessionStore.Save(req, responseRecorder2, session2)
	assert.NoError(t, err)

	// Create new request with the session cookie
	cookies2 := responseRecorder2.Result().Cookies()
	assert.NotEmpty(t, cookies2, "Should have session cookie")

	newReq2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range cookies2 {
		newReq2.AddCookie(cookie)
	}

	// Try to get the saved session
	savedSession2, err := sessionStore.Get(newReq2, "marko")
	assert.NoError(t, err)
	assert.NotNil(t, savedSession2)
	assert.False(t, savedSession2.IsNew)

	// Verify user ID in retrieved session
	savedUserID2, ok := savedSession2.Values["user_id"].(string)
	assert.True(t, ok)
	assert.Equal(t, mockUser.ID.Hex(), savedUserID2)

}
