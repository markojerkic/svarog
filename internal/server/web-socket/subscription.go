package websocket

import (
	"sync"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/types"
)

type Subscription[T interface{}] interface {
	GetSubscriptionId() string
	GetUpdates() <-chan T
	RemoveInstance(instanceId string)
	AddInstance(instanceId string)
	GetClientId() string
	Notify(T)
	Close()
}

type LogSubscription struct {
	id              string
	hub             *WatchHub[types.StoredLog]
	clientId        string
	updates         chan types.StoredLog
	clientInstances map[string]bool
	isClosed        bool
	mutex           *sync.Mutex
}

// AddInstance implements Subscription.
func (l *LogSubscription) AddInstance(instanceId string) {
	panic("unimplemented")
}

// RemoveInstance implements Subscription.
func (l *LogSubscription) RemoveInstance(instanceId string) {
	panic("unimplemented")
}

// GetUpdates implements Subscription.
func (self *LogSubscription) GetUpdates() <-chan types.StoredLog {
	return self.updates
}

func (self *LogSubscription) GetClientId() string {
	return self.clientId
}

func (self *LogSubscription) Notify(log types.StoredLog) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	if self.isClosed {
		return
	}

	self.updates <- log
}

func (self *LogSubscription) Close() {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	if self.isClosed {
		return
	}
	close(self.updates)
	self.isClosed = true
}

func (self *LogSubscription) GetSubscriptionId() string {
	return self.id
}

var _ Subscription[types.StoredLog] = &LogSubscription{}

func createSubscription(clientId string) Subscription[types.StoredLog] {
	return &LogSubscription{
		id:              uuid.New().String(),
		hub:             &LogsHub,
		clientId:        clientId,
		updates:         make(chan types.StoredLog, 100),
		clientInstances: make(map[string]bool),
		isClosed:        false,
		mutex:           &sync.Mutex{},
	}
}
