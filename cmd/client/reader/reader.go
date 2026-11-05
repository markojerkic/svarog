package reader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sync"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/rpc"
)

type Reader interface {
	Run(context.Context, *sync.WaitGroup)
}

type ReaderImpl struct {
	input      *bufio.Reader
	output     chan<- *rpc.LogLine
	instanceId string
}

const (
	// Leave headroom for JSON encoding under NATS's default 1MiB payload limit.
	maxLogLineBytes = 512 * 1024
	truncatedSuffix = " [truncated]"
)

var (
	errLineTooLong = errors.New("log line exceeds size limit")
	ansiRegex      = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")
)

func (r *ReaderImpl) Run(_ context.Context, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	sequence := 0
	for {
		line, err := readLine(r.input)
		if errors.Is(err, errLineTooLong) {
			slog.Error("Log line exceeded the size limit and was truncated", "limit", maxLogLineBytes)
		} else if err != nil {
			if !errors.Is(err, io.EOF) {
				slog.Error("Failed to read log line", "err", err)
			}
			return
		}

		fmt.Println(line)
		line = ansiRegex.ReplaceAllString(line, "")
		r.output <- &rpc.LogLine{
			Message:    line,
			Timestamp:  time.Now(),
			Sequence:   sequence,
			InstanceId: r.instanceId,
		}
		sequence = (sequence + 1) % math.MaxInt64
	}
}

// Scanner stops at 64KiB and cannot resume, so a long line would drop the rest of the stream.
func readLine(input *bufio.Reader) (string, error) {
	var buf []byte
	tooLong := false
	for {
		chunk, isPrefix, err := input.ReadLine()
		if err != nil && (!errors.Is(err, io.EOF) || len(buf) == 0) {
			return "", err
		}
		room := maxLogLineBytes - len(buf)
		if len(chunk) > room {
			tooLong = true
			chunk = chunk[:room]
		}
		buf = append(buf, chunk...)
		if isPrefix {
			continue
		}
		if tooLong {
			return string(buf[:maxLogLineBytes-len(truncatedSuffix)]) + truncatedSuffix, errLineTooLong
		}
		return string(buf), nil
	}
}

func NewReader(input io.Reader, output chan<- *rpc.LogLine, instanceId string) Reader {
	return &ReaderImpl{
		input:      bufio.NewReader(input),
		output:     output,
		instanceId: instanceId,
	}
}
