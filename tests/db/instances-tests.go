package db

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (self *LogsCollectionRepositorySuite) TestInstances() {
	t := self.T()

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
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::2",
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
	err := self.logService.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	instances, err := self.logService.GetInstances(context.Background(), "marko")
	assert.NoError(t, err)

	sort.Sort(sort.StringSlice(instances))

	assert.Equal(t, 2, len(instances), fmt.Sprintf("Expected 2 instances, got %+v", instances))
	assert.Equal(t, []string{"::1", "::2"}, instances)

}

func (self *LogsCollectionRepositorySuite) TestFilterByInstances() {
	t := self.T()

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
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::2",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}
	err := self.logService.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	logs, err := self.logService.GetLogs(context.Background(), "marko", &[]string{"::1"}, 10, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(logs))
	for _, log := range logs {
		assert.Equal(t, "::1", log.Client.IpAddress)

	}
}

func (self *LogsCollectionRepositorySuite) TestFilterByMultipleInstances() {
	t := self.T()

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
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::2",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::3",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::2",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}
	err := self.logService.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	logs, err := self.logService.GetLogs(context.Background(), "marko", &[]string{"::1", "::2"}, 10, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(logs))
	for _, log := range logs {
		assert.NotEqual(t, "::3", log.Client.IpAddress)

	}
}

func (self *LogsCollectionRepositorySuite) TestFilterByAllInstances() {
	t := self.T()

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
				ClientId:  "marko",
				IpAddress: "::1",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::2",
			},
			Timestamp: time.Now(),
			LogLine:   "marko",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::3",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
		{
			Client: types.StoredClient{
				ClientId:  "marko",
				IpAddress: "::2",
			},
			Timestamp: time.Now(),
			LogLine:   "jerkić",
		},
	}
	err := self.logService.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	logs, err := self.logService.GetLogs(context.Background(), "marko", &[]string{"::1", "::2", "::3"}, 10, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(mockLogLines), len(logs))
}
