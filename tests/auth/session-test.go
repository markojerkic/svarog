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
}
