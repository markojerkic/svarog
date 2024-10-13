package websocket

import (
	"sync"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/types"
)

type Subscription interface {
	GetSubscriptionId() string
	GetUpdates() <-chan types.StoredLog
	SetInstances(instances []string)
	GetClientId() string
	Notify(types.StoredLog)
	Close()
}

type LogSubscription struct {
	id              string
	hub             *WatchHub
	clientId        string
	updates         chan types.StoredLog
	clientInstances map[string]bool
	isClosed        bool
	mutex           *sync.Mutex

	selectedInstances map[string]bool
}

// SetInstances implements Subscription.
func (self *LogSubscription) SetInstances(instances []string) {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.selectedInstances = make(map[string]bool)
	for _, instance := range instances {
		self.selectedInstances[instance] = true
	}
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

	if len(self.selectedInstances) > 0 && !self.selectedInstances[log.Client.IpAddress] {
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

var _ Subscription = &LogSubscription{}

func createSubscription(clientId string) Subscription {
	return &LogSubscription{
		id:                uuid.New().String(),
		hub:               &LogsHub,
		clientId:          clientId,
		updates:           make(chan types.StoredLog, 100),
		clientInstances:   make(map[string]bool),
		isClosed:          false,
		selectedInstances: make(map[string]bool),
		mutex:             &sync.Mutex{},
	}
}
