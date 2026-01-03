package db

import (
	"context"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (s *LogsCollectionRepositorySuite) TestAddClient() {
	t := s.T()

	mockLogLines := []types.StoredLog{
		{
			Client: types.StoredClient{
				ProjectId:  "test-project",
				ClientId:   "marko",
				InstanceId: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ProjectId:  "test-project",
				ClientId:   "jerkić",
				InstanceId: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}

	err := s.logService.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	// Verify logs were saved by retrieving them
	logPage, err := s.logService.GetLogs(context.Background(), db.LogPageRequest{
		ProjectId: "test-project",
		ClientId:  "marko",
		Instances: nil,
		PageSize:  10,
		LogLineId: nil,
		Cursor:    nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(logPage.Logs))

	logPage, err = s.logService.GetLogs(context.Background(), db.LogPageRequest{
		ProjectId: "test-project",
		ClientId:  "jerkić",
		Instances: nil,
		PageSize:  10,
		LogLineId: nil,
		Cursor:    nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(logPage.Logs))
}
