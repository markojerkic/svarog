package auth

import (
	"context"
	"log/slog"

	authlayer "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestRegisterNewUser() {
	t := suite.T()

	err := suite.authService.Register(context.Background(), "marko", "marko")
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

	err := suite.authService.Register(context.Background(), "marko", "marko")
	assert.NoError(t, err)

	err = suite.authService.Register(context.Background(), "marko", "marko")
	assert.Error(t, err)
}
