package websocket

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	ws "github.com/markojerkic/svarog/internal/server/web-socket"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSubscribe(t *testing.T) {
	markoSubscription := ws.LogsHub.Subscribe("marko")
	jerkicSubscription := ws.LogsHub.Subscribe("jerkic")

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
		LogLine:   "jerkic",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "jerkic",
			IpAddress: "::1",
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

	jerkicUpdates := make([]types.StoredLog, 0, 10)
	timeout, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	for {
		isDone := false
		select {
		case log, ok := <-(*jerkicSubscription).GetUpdates():
			isDone = !ok
			jerkicUpdates = append(jerkicUpdates, log)
		case <-timeout.Done():
			isDone = true
		}
		if isDone {
			break
		}
	}

	assert.Equal(t, 1, len(markoUpdates), "Expected 1 log line for marko")
	assert.Equal(t, 1, len(jerkicUpdates), "Expected 1 log line for jerkic")

	assert.Equal(t, "marko", markoUpdates[0].LogLine, "Expected log line to be marko")
	assert.Equal(t, "jerkic", jerkicUpdates[0].LogLine, "Expected log line to be jerkic")
}

func TestUnsubscribe(t *testing.T) {
	subscription := ws.LogsHub.Subscribe("marko")
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

	markoUpdates := make([]types.StoredLog, 0, 10)

	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	for {
		isDone := false
		select {
		case log, ok := <-(*subscription).GetUpdates():
			isDone = !ok
			if ok {
				markoUpdates = append(markoUpdates, log)
			}
		case <-timeout.Done():
			isDone = true
		}
		if isDone {
			break
		}
	}
	assert.Equal(t, 1, len(markoUpdates), "Expected 1 log line for marko")

	log.Print("Unsubscribing")
	ws.LogsHub.Unsubscribe(subscription)
	log.Print("Unsubscribing again")
	ws.LogsHub.Unsubscribe(subscription)

	timeout, cancel = context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	for {
		isDone := false
		select {
		case log, ok := <-(*subscription).GetUpdates():
			isDone = !ok
			if ok {
				markoUpdates = append(markoUpdates, log)
			}
		case <-timeout.Done():
			isDone = true
		}
		if isDone {
			break
		}
	}
	assert.Equal(t, 1, len(markoUpdates), "Expected 1 log line for marko after unsubscribing")

}

type subscriptionTestBed struct {
	t            *testing.T
	subscription *ws.Subscription
	logs         []types.StoredLog
}

func (self *subscriptionTestBed) run() {
	timeout, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	for {
		isDone := false
		select {
		case log, ok := <-(*self.subscription).GetUpdates():
			isDone = !ok
			if ok {
				self.logs = append(self.logs, log)
			}
		case <-timeout.Done():
			isDone = true
		}
		if isDone {
			break
		}
	}
}

func (self *subscriptionTestBed) assert(numLogs int) {
	assert.Equal(self.t, numLogs, len(self.logs))
	assert.Equal(self.t, "marko", self.logs[0].LogLine, "Expected log line to be marko")
}

func TestSubscribeTwice(t *testing.T) {
	markoSubscription := ws.LogsHub.Subscribe("marko")
	marko2Subscription := ws.LogsHub.Subscribe("marko")

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
		LogLine:   "jerkic",
		Timestamp: time.Now(),
		Client: types.StoredClient{
			ClientId:  "jerkic",
			IpAddress: "::1",
		},
		SequenceNumber: 0,
	})

	testBed := subscriptionTestBed{
		t:            t,
		subscription: markoSubscription,
		logs:         make([]types.StoredLog, 0, 10),
	}
	testBed2 := subscriptionTestBed{
		t:            t,
		subscription: marko2Subscription,
		logs:         make([]types.StoredLog, 0, 10),
	}

	testBed.run()
	testBed2.run()
	testBed.assert(1)
	testBed2.assert(1)

	ws.LogsHub.Unsubscribe(marko2Subscription)

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

	testBed.run()
	testBed2.run()
	testBed.assert(2)
	testBed2.assert(1)

}
