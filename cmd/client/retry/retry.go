package retry

import (
	"context"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
)

type RetryService interface {
	Run(context.Context, chan<- []rpc.LogLine)
	RetryChan() <-chan []rpc.LogLine
}

type Retry struct {
	backlogInput <-chan []rpc.LogLine
	pollInterval int

	backlog []rpc.LogLine
}

var _ RetryService = &Retry{}

func (r *Retry) RetryChan() <-chan []rpc.LogLine {
	return r.backlogInput
}

func (r *Retry) Run(ctx context.Context, output chan<- []rpc.LogLine) {
	for {
		select {
		case <-ctx.Done():
			return
		case lines := <-r.backlogInput:
			r.backlog = append(r.backlog, lines...)
		case <-time.After(time.Duration(r.pollInterval) * time.Second):
			output <- r.backlog
		}

	}
}

func NewRetry(backlog <-chan []rpc.LogLine, pollInterval int) RetryService {
	return &Retry{
		backlogInput: backlog,
		pollInterval: pollInterval,

		backlog: make([]rpc.LogLine, 0, 1000),
	}
}
