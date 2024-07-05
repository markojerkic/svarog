package db

import (
	"sync"
)

type Backlog interface {
	getLogs() <-chan []interface{}
	addToBacklog(log interface{})
	addAllToBacklog(logs []interface{})
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

func (self *IBacklog) dump(index int) {
	self.dumpLock.Lock()
	defer self.dumpLock.Unlock()

	logsToBeDumped := make([]interface{}, index)
	copy(logsToBeDumped, self.workingLogs[0:index])

	if len(logsToBeDumped) == 0 {
		return
	}

	self.backlog <- logsToBeDumped
}

func (self *IBacklog) forceDump() {
	self.Lock()
	defer self.Unlock()

	self.dump(self.index)
	self.index = 0
}

func (self *IBacklog) addToBacklog(log interface{}) {
	self.Lock()
	defer self.Unlock()

	self.workingLogs[self.index] = log
	self.index = (self.index + 1) % backlogLimit

	if self.index == 0 {
		self.dump(backlogLimit)
	}

}

func (self *IBacklog) addAllToBacklog(logs []interface{}) {
	self.Lock()
	defer self.Unlock()

	for _, log := range logs {
		self.workingLogs[self.index] = log
		self.index = (self.index + 1) % backlogLimit
	}

	self.forceDump()
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

func newBacklog(backlogSize int) Backlog {
	return &IBacklog{
		sync.Mutex{},
		sync.Mutex{},
		make([]interface{}, backlogLimit),
		make(chan []interface{}, backlogSize),
		0,
	}
}
