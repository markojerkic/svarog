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

	firstId := primitive.NewObjectID()
	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        firstId,
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::1",
		},
		SequenceNumber: 0,
	})

	secondId := primitive.NewObjectID()
	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        secondId,
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::2",
		},
		SequenceNumber: 0,
	})
	(*markoSubscription).RemoveInstance("::2")

	thirdId := primitive.NewObjectID()
	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        thirdId,
		LogLine:   "marko",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "marko",
			IpAddress: "::1",
		},
		SequenceNumber: 0,
	})

	fourthId := primitive.NewObjectID()
	ws.LogsHub.NotifyInsert(types.StoredLog{
		ID:        fourthId,
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

	assert.Equal(t, 3, len(markoUpdates))

	assert.Equal(t, firstId, markoUpdates[0].ID)
	assert.Equal(t, "::1", markoUpdates[0].Client.IpAddress)
	assert.Equal(t, secondId, markoUpdates[1].ID)
	assert.Equal(t, "::2", markoUpdates[1].Client.IpAddress)
	assert.Equal(t, thirdId, markoUpdates[2].ID)
	assert.Equal(t, "::1", markoUpdates[2].Client.IpAddress)

}
