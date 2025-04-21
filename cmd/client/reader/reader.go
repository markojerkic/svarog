package reader

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"regexp"
	"sync"
	"time"

	rpc "github.com/markojerkic/svarog/internal/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Reader interface {
	hasNext() bool
	next() (string, error)
	Run(context.Context, *sync.WaitGroup)
}

type Line struct {
	LogLine   string
	IsError   bool
	Timestamp time.Time
}

type ReaderImpl struct {
	input    *bufio.Scanner
	file     *os.File
	output   chan *rpc.LogLine
	fileName string

	clientId string
}

const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiRegex = regexp.MustCompile(ansi)

func (r *ReaderImpl) Run(ctx context.Context, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	var logLine *rpc.LogLine
	i := 0
	for r.hasNext() {
		line, err := r.next()
		timestamp := time.Now()
		var message string
		if err != nil {
			message = err.Error()
		} else {
			message = line
		}

		fmt.Println(message)
		message = ansiRegex.ReplaceAllString(message, "")
		logLine = &rpc.LogLine{
			Client:    r.clientId,
			Message:   message,
			Timestamp: timestamppb.New(timestamp),
			Sequence:  int64(i),
		}
		r.output <- logLine
		i = (i + 1) % math.MaxInt64
	}

}

// hasNext implements Reader.
func (r *ReaderImpl) hasNext() bool {
	return r.input.Scan()
}

// next implements Reader.
func (r *ReaderImpl) next() (string, error) {
	if err := r.input.Err(); err != nil {
		return "", err
	}
	return r.input.Text(), nil
}

func NewReader(input *os.File, clientId string, output chan *rpc.LogLine) Reader {
	return &ReaderImpl{bufio.NewScanner(input),
		input,
		output,
		input.Name(),
		clientId,
	}
}
