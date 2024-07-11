package websocket

import (
	"sync"

	"github.com/markojerkic/svarog/internal/server/db"
)

type WatchHub interface {
	Subscribe(clientId string) *Subscription[db.StoredLog]
	Unsubscribe(*Subscription[db.StoredLog])
	NotifyInsert(db.StoredLog)
	NotifyInsertMultiple([]db.StoredLog)
}

type subscriptions map[*Subscription[db.StoredLog]]bool
type LogsWatchHub struct {
	mutex    sync.Mutex
	channels map[string]subscriptions
}

// Subscribe implements WatchHub.
func (self *LogsWatchHub) Subscribe(clientId string) *Subscription[db.StoredLog] {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	subscription := Subscribe(clientId)
	if self.channels[clientId] == nil {
		self.channels[clientId] = make(subscriptions)
	}
	self.channels[clientId][&subscription] = true
	return &subscription
}

// Unsubscribe implements WatchHub.
func (self *LogsWatchHub) Unsubscribe(subscription *Subscription[db.StoredLog]) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	clientId := (*subscription).getClientId()
	if self.channels[clientId] == nil {
		return
	}
	subscriptions := self.channels[clientId]
	delete(subscriptions, subscription)
}

// NotifyInsert implements WatchHub.
func (self *LogsWatchHub) NotifyInsert(logLine db.StoredLog) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	clientId := logLine.Client.ClientId
	if self.channels[clientId] == nil {
		return
	}
	subscriptions := self.channels[clientId]
	for subscription := range subscriptions {
		(*subscription).notify(logLine)
	}
}

// NotifyInsert implements WatchHub.
func (self *LogsWatchHub) notify(logLine db.StoredLog) {
	clientId := logLine.Client.ClientId
	if self.channels[clientId] == nil {
		return
	}
	subscriptions := self.channels[clientId]
	for subscription := range subscriptions {
		(*subscription).notify(logLine)
	}
}

// NotifyInsertMultiple implements WatchHub.
func (self *LogsWatchHub) NotifyInsertMultiple(lines []db.StoredLog) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	for _, logLine := range lines {
		self.notify(logLine)
	}
}

var LogsHub WatchHub = &LogsWatchHub{
	mutex:    sync.Mutex{},
	channels: make(map[string]subscriptions),
}
