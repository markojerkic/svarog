package auth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestAuthSuite(t *testing.T) {
	suite.Run(t, new(AuthSuite))
}
