package archive

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestArchiveSuite(t *testing.T) {
	suite.Run(t, new(ArchiveSuite))
}
