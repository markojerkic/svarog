package db

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// Run test duite
func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(LogsCollectionRepositorySuite))
}
