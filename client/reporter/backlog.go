package reporter

import (
	"log"
	"time"
)

type Backlog[T any] interface {
	addToBacklog(T)
	dumpBacklog()
}

type BacklogImpl[T any] struct {
	backlog       []T
	dump          func([]T) error
	dumpIsRunning bool
}

var _ Backlog[any] = (*BacklogImpl[any])(nil)

// addToBacklog implements Backlog.
func (b *BacklogImpl[T]) addToBacklog(item T) {
	b.backlog = append(b.backlog, item)
	if !b.dumpIsRunning {
		go b.dumpBacklog()
	}
}

func (b *BacklogImpl[T]) attemptDumpBacklog() {
	if b.dumpIsRunning {
		return
	}

	b.dumpIsRunning = true
	defer func() {
		b.dumpIsRunning = false
	}()

	for {
		if len(b.backlog) == 0 {
			return
		}
		err := b.dump(b.backlog)

		if err == nil {
			b.backlog = b.backlog[:0]
			return
		}

		log.Printf("Error dumping backlog: %v\n", err)
		log.Println("Retrying dumping backlog after 2s")
		time.Sleep(2 * time.Second)
	}
}

// dumpBacklog implements Backlog.
func (b *BacklogImpl[T]) dumpBacklog() {
	b.attemptDumpBacklog()
}

func NewBacklog[T any](dump func([]T) error) *BacklogImpl[T] {
	return &BacklogImpl[T]{
		dump:    dump,
		backlog: make([]T, 0),

		dumpIsRunning: false,
	}
}
