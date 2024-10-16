package websocket

import (
	"sync"

	"github.com/markojerkic/svarog/internal/server/types"
)

type WatchHub interface {
	Subscribe(clientId string) *Subscription
	Unsubscribe(*Subscription)
	NotifyInsert(types.StoredLog)
	NotifyInsertMultiple([]types.StoredLog)
}

type subscriptions map[*Subscription]bool
type LogsWatchHub struct {
	mutex    sync.Mutex
	channels map[string]subscriptions
}

var _ WatchHub = &LogsWatchHub{}

// Subscribe implements WatchHub.
func (self *LogsWatchHub) Subscribe(clientId string) *Subscription {
	subscription := createSubscription(clientId)

	self.mutex.Lock()
	if self.channels[clientId] == nil {
		self.channels[clientId] = make(subscriptions)
	}
	self.channels[clientId][&subscription] = true
	self.mutex.Unlock()

	return &subscription
}

// Unsubscribe implements WatchHub.
func (self *LogsWatchHub) Unsubscribe(subscription *Subscription) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	clientId := (*subscription).GetClientId()

	if self.channels[clientId] == nil {
		return
	}
	subscriptions := self.channels[clientId]
	(*subscription).Close()

	for sub := range subscriptions {
		if (*sub).GetSubscriptionId() == (*subscription).GetSubscriptionId() {
			delete(subscriptions, sub)
			break
		}
	}

}

// NotifyInsert implements WatchHub.
func (self *LogsWatchHub) NotifyInsert(logLine types.StoredLog) {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	clientId := logLine.Client.ClientId
	if self.channels[clientId] == nil {
		return
	}

	subscriptions := self.channels[clientId]
	for subscription := range subscriptions {
		(*subscription).Notify(logLine)
	}
}

// NotifyInsert implements WatchHub.
func (self *LogsWatchHub) notify(logLine types.StoredLog) {
	clientId := logLine.Client.ClientId
	if self.channels[clientId] == nil {
		return
	}
	subscriptions := self.channels[clientId]
	for subscription := range subscriptions {
		(*subscription).Notify(logLine)
	}
}

// NotifyInsertMultiple implements WatchHub.
func (self *LogsWatchHub) NotifyInsertMultiple(lines []types.StoredLog) {
	for _, logLine := range lines {
		self.notify(logLine)
	}
}

var LogsHub WatchHub = &LogsWatchHub{
	mutex:    sync.Mutex{},
	channels: make(map[string]subscriptions),
}
