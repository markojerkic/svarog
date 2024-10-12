package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	ws "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAddInstance(t *testing.T) {
	markoSubscription := ws.LogsHub.Subscribe("marko")

	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        primitive.NewObjectID(),
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::1",
		},
		SequenceNumber: 0,
	})

	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        primitive.NewObjectID(),
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::2",
		},
		SequenceNumber: 0,
	})

	markoUpdates := make([]types.StoredLog, 0, 10)
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	for {
		isDone := false
		select {
		case log, ok := <-(*markoSubscription).GetUpdates():
			isDone = !ok
			markoUpdates = append(markoUpdates, log)
		case <-timeout.Done():
			isDone = true
		}
		if isDone {
			break
		}
	}

	assert.Equal(t, 2, len(markoUpdates))

	markoUpdates = make([]types.StoredLog, 0, 10)
	(*markoSubscription).RemoveInstance("::2")

	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        primitive.NewObjectID(),
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::1",
		},
		SequenceNumber: 0,
	})

	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        primitive.NewObjectID(),
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::2",
		},
		SequenceNumber: 0,
	})

	assert.Equal(t, 1, len(markoUpdates))
	assert.Equal(t, "::1", markoUpdates[0].Client.IpAddress)

}
