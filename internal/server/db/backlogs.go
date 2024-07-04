package db

import (
	"sync"
)

type Backlog interface {
	getLogs() <-chan []interface{}
	addToBacklog(log interface{})
	forceDump()
	isEmpty() bool
	isFull() bool
	count() int
}

type IBacklog struct {
	sync.Mutex
	dumpLock sync.Mutex

	workingLogs []interface{}
	backlog     chan []interface{}

	index int
}

var _ Backlog = &IBacklog{}

var backlogLimit = 1000

func (self *IBacklog) getLogs() <-chan []interface{} {
	self.Lock()
	defer self.Unlock()

	return self.backlog
}

func (self *IBacklog) count() int {
	self.Lock()
	defer self.Unlock()

	return self.index
}

func (self *IBacklog) forceDump() {
	self.dumpLock.Lock()
	defer self.dumpLock.Unlock()

	logsToBeDumped := make([]interface{}, self.index)
	copy(logsToBeDumped, self.workingLogs[:self.index])

	if len(logsToBeDumped) == 0 {
		return
	}

	self.backlog <- logsToBeDumped
	self.index = 0
}

func (self *IBacklog) addToBacklog(log interface{}) {
	self.Lock()
	defer self.Unlock()

	self.workingLogs[self.index] = log

	if self.index == backlogLimit-1 {
		self.forceDump()
	}
	self.index = (self.index + 1) % backlogLimit

}

func (self *IBacklog) isEmpty() bool {
	self.Lock()
	defer self.Unlock()

	return self.index == 0
}

func (self *IBacklog) isFull() bool {
	self.Lock()
	defer self.Unlock()

	return self.index == backlogLimit-1
}

func newBacklog() Backlog {
	return &IBacklog{
		sync.Mutex{},
		sync.Mutex{},
		make([]interface{}, backlogLimit),
		make(chan []interface{}, 1024*1024),
		0,
	}
}
