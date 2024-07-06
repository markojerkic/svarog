package db

import (
	"sync"
)

type Backlog[T interface{}] interface {
	GetLogs() <-chan []T
	AddToBacklog(log T)
	AddAllToBacklog(logs []T)
	ForceDump()
	IsEmpty() bool
	IsFull() bool
	Count() int
}

type IBacklog[T interface{}] struct {
	sync.Mutex
	dumpLock sync.Mutex

	workingLogs []T
	backlog     chan []T

	index int
}

var _ Backlog[interface{}] = &IBacklog[interface{}]{}

var backlogLimit = 1000

func (self *IBacklog[T]) GetLogs() <-chan []T {
	self.Lock()
	defer self.Unlock()

	return self.backlog
}

func (self *IBacklog[T]) Count() int {
	self.Lock()
	defer self.Unlock()

	return self.index
}

func (self *IBacklog[T]) dump(index int) {
	self.dumpLock.Lock()
	defer self.dumpLock.Unlock()

	logsToBeDumped := make([]T, index)
	copy(logsToBeDumped, self.workingLogs[0:index])

	if len(logsToBeDumped) == 0 {
		return
	}

	self.backlog <- logsToBeDumped
}

func (self *IBacklog[T]) ForceDump() {
	self.Lock()
	defer self.Unlock()

	self.dump(self.index)
	self.index = 0
}

func (self *IBacklog[T]) AddToBacklog(log T) {
	self.Lock()
	defer self.Unlock()

	self.workingLogs[self.index] = log
	self.index = (self.index + 1) % backlogLimit

	if self.index == 0 {
		self.dump(backlogLimit)
	}

}

func (self *IBacklog[T]) AddAllToBacklog(logs []T) {
	self.Lock()
	defer self.Unlock()

	for _, log := range logs {
		self.workingLogs[self.index] = log
		self.index = (self.index + 1) % backlogLimit
	}

	self.ForceDump()
}

func (self *IBacklog[T]) IsEmpty() bool {
	self.Lock()
	defer self.Unlock()

	return self.index == 0
}

func (self *IBacklog[T]) IsFull() bool {
	self.Lock()
	defer self.Unlock()

	return self.index == backlogLimit-1
}

func NewBacklog[T interface{}](backlogSize int) Backlog[T] {
	return &IBacklog[T]{
		sync.Mutex{},
		sync.Mutex{},
		make([]T, backlogLimit),
		make(chan []T, backlogSize),
		0,
	}
}
