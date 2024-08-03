package websocket

import (
	"github.com/markojerkic/svarog/internal/server/types"
)

type Subscription[T interface{}] interface {
	GetUpdates() <-chan T
	RemoveInstance(instanceId string)
	AddInstance(instanceId string)
	GetClientId() string
	Notify(T)
	Close()
}

type LogSubscription struct {
	hub             *WatchHub[*types.StoredLog]
	clientId        string
	updates         chan *types.StoredLog
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
func (self *LogSubscription) GetUpdates() <-chan *types.StoredLog {
	return self.updates
}

func (self *LogSubscription) GetClientId() string {
	return self.clientId
}

func (self *LogSubscription) Notify(log *types.StoredLog) {
	self.updates <- log
}

func (self *LogSubscription) Close() {
	close(self.updates)
}

var _ Subscription[*types.StoredLog] = &LogSubscription{}

func Subscribe(clientId string) Subscription[*types.StoredLog] {
	return &LogSubscription{
		hub:             &LogsHub,
		clientId:        clientId,
		updates:         make(chan *types.StoredLog, 100),
		clientInstances: make(map[string]bool),
	}
}
