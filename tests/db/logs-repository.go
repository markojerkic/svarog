package db

import (
	"fmt"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/assert"
)

func (s *RepositorySuite) TestNoClients() {
	t := s.T()

	clients, err := s.mongoRepository.GetClients()

	assert.NoError(t, err)

	assert.Equal(t, 0, len(clients))
}

func (s *RepositorySuite) TestAddClient() {
	t := s.T()

	mockLogLines := []db.StoredLog{
		db.StoredLog{
			Client: db.StoredClient{
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		db.StoredLog{
			Client: db.StoredClient{
				ClientId:  "jerkić",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}

	err := s.mongoRepository.SaveLogs(mockLogLines)
	assert.NoError(t, err)

	clients, err := s.mongoRepository.GetClients()

	assert.NoError(t, err)

	assert.Equal(t, 2, len(clients), fmt.Sprintf("Expected 2 client, got %+v", clients))

	clientIdsSet := make(map[string]bool)
	for _, client := range clients {
		clientIdsSet[client.Client.ClientId] = true
	}

	assert.Equal(t, 2, len(clientIdsSet), fmt.Sprintf("Expected 2 client, got %+v", clientIdsSet))
	assert.Equal(t, true, clientIdsSet["marko"])
	assert.Equal(t, true, clientIdsSet["jerkić"])
}
