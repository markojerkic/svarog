package reader

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
	"unicode/utf8"

	"github.com/markojerkic/svarog/internal/lib/svarogclient"
)

type Reader struct {
	input   *bufio.Reader
	enqueue func(svarogclient.Entry) error
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

func (r *Reader) Run(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := readLine(r.input)
		if errors.Is(err, errLineTooLong) {
			fmt.Fprintln(os.Stderr, "Log line exceeded the size limit and was truncated; limit:", maxLogLineBytes)
		} else if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read log line: %w", err)
		}

		fmt.Println(line)
		line = ansiRegex.ReplaceAllString(line, "")
		if err := r.enqueue(svarogclient.Entry{Message: line, Timestamp: time.Now()}); err != nil {
			if errors.Is(err, svarogclient.ErrInvalidEntry) {
				// Invalid source data was previously rejected by ingest. Keep
				// reading valid lines after the client's local rejection too.
				continue
			}
			return fmt.Errorf("enqueue log line: %w", err)
		}
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
			cut := maxLogLineBytes - len(truncatedSuffix)
			for cut > 0 && !utf8.RuneStart(buf[cut]) {
				cut--
			}
			return string(buf[:cut]) + truncatedSuffix, errLineTooLong
		}
		return string(buf), nil
	}
}

func NewReader(input io.Reader, enqueue func(svarogclient.Entry) error) *Reader {
	return &Reader{
		input:   bufio.NewReader(input),
		enqueue: enqueue,
	}
}
