package websocket

import (
	"github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/markojerkic/svarog/tests/testutils"
)

type WatchHubSuite struct {
	testutils.BaseSuite

	watchHub *websocket.WatchHub
}

func (s *WatchHubSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	s.watchHub = s.WatchHub
}

func (s *WatchHubSuite) TearDownSuite() {
	s.BaseSuite.TearDownSuite()
}
