package db

import (
	"context"
	"fmt"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (s *LogsCollectionRepositorySuite) TestNoClients() {
	t := s.T()

	clients, err := s.logService.GetClients(context.Background())

	assert.NoError(t, err)

	assert.Equal(t, 0, len(clients))
}

func (s *LogsCollectionRepositorySuite) TestAddClient() {
	t := s.T()

	mockLogLines := []types.StoredLog{
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "jerkić",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}

	err := s.logService.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	clients, err := s.logService.GetClients(context.Background())

	assert.NoError(t, err)

	assert.Equal(t, 2, len(clients), fmt.Sprintf("Expected 2 client, got %+v", clients))

	clientIdsSet := make(map[string]bool)
	for _, client := range clients {
		clientIdsSet[client.ClientId] = true
	}

	assert.Equal(t, 2, len(clientIdsSet), fmt.Sprintf("Expected 2 client, got %+v", clientIdsSet))
	assert.Equal(t, true, clientIdsSet["marko"])
	assert.Equal(t, true, clientIdsSet["jerkić"])
}
