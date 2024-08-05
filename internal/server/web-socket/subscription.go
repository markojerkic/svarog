package websocket

import (
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
	self.updates <- log
}

func (self *LogSubscription) Close() {
	_, ok := <-self.updates
	if ok {
		close(self.updates)
	}
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
	}
}
