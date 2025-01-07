package serverauth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestServerauthSuite(t *testing.T) {
	suite.Run(t, new(ServerauthSuite))
}
