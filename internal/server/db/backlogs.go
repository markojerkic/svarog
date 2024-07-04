package db

import (
	"log/slog"
	"sync"
)

type Backlog struct {
	sync.RWMutex
	logs  []interface{}
	index int
}

var backlogLimit = 1024 * 1024

func (self *Backlog) getLogs() []interface{} {
	self.Lock()
	defer self.Unlock()

	logs := self.logs[:self.index]
	self.index = 0
	slog.Debug("Clearing backlog")
	return logs
}

func (self *Backlog) add(log interface{}) {
	self.Lock()
	defer self.Unlock()

	self.logs[self.index] = log
	self.index = (self.index + 1) % backlogLimit
}

func (self *Backlog) isEmpty() bool {
	self.Lock()
	defer self.Unlock()

	return self.index == 0
}

func (self *Backlog) isFull() bool {
	self.Lock()
	defer self.Unlock()

	return self.index == backlogLimit-1
}

func newBacklog() *Backlog {
	return &Backlog{
		sync.RWMutex{},
		make([]interface{}, backlogLimit),
		0,
	}
}
