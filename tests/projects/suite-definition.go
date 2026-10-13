package projects

import (
	"context"

	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

type ProjectsSuite struct {
	testutils.BaseSuite
}

// SetupSuite implements suite.SetupAllSuite.
func (p *ProjectsSuite) SetupSuite() {
	p.BaseSuite.SetupSuite()
}

// TearDownSubTest implements suite.TearDownSubTest.
func (p *ProjectsSuite) TearDownSubTest() {
	_, err := p.PostgresPool.Exec(context.Background(), "TRUNCATE projects, users CASCADE")
	require.NoError(p.T(), err, "clear PostgreSQL projects, clients, users, and memberships between project tests")
}

// TearDownTest implements suite.TearDownTestSuite.
func (p *ProjectsSuite) TearDownTest() {
	p.TearDownSubTest()
}

// TearDownSuite implements suite.TearDownAllSuite.
func (p *ProjectsSuite) TearDownSuite() {
	p.BaseSuite.TearDownSuite()
}
