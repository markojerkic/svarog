package websocket

import "github.com/markojerkic/svarog/internal/server/db"

type Subscription[T interface{}] interface {
	GetUpdates() <-chan T
	RemoveInstance(instanceId string)
	AddInstance(instanceId string)
	getClientId() string
	notify(T)
}

type LogSubscription struct {
	hub             *WatchHub
	clientId        string
	updates         chan db.StoredLog
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
func (self *LogSubscription) GetUpdates() <-chan db.StoredLog {
	return self.updates
}

func (self *LogSubscription) getClientId() string {
	return self.clientId
}

func (self *LogSubscription) notify(log db.StoredLog) {
	self.updates <- log
}

var _ Subscription[db.StoredLog] = &LogSubscription{}

func Subscribe(clientId string) Subscription[db.StoredLog] {
	return &LogSubscription{
		hub:             &LogsHub,
		clientId:        clientId,
		updates:         make(chan db.StoredLog),
		clientInstances: make(map[string]bool),
	}
}
