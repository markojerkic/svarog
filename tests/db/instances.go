package db

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (self *RepositorySuite) TestInstances() {
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
	err := self.mongoRepository.SaveLogs(context.Background(), mockLogLines)
	assert.NoError(t, err)

	instances, err := self.mongoRepository.GetInstances(context.Background(), "marko")
	assert.NoError(t, err)

	sort.Sort(sort.StringSlice(instances))

	assert.Equal(t, 2, len(instances), fmt.Sprintf("Expected 2 instances, got %+v", instances))
	assert.Equal(t, []string{"::1", "::2"}, instances)

}
