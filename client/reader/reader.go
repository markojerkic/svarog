package reader

import (
	"bufio"
	"os"
	"time"
)

type Reader interface {
	hasNext() bool
	next() (string, error)
	Run(chan bool)
}

type Line struct {
	LogLine   string
	IsError   bool
	Timestamp time.Time
}

type ReaderImpl struct {
	input  *bufio.Scanner
	output chan *Line
}

func (r *ReaderImpl) Run(stopSignal chan bool) {
	defer close(r.output)
	for r.hasNext() {
		line, err := r.next()
		timestamp := time.Now()
		if err != nil {
			r.output <- &Line{LogLine: err.Error(), IsError: true, Timestamp: timestamp}
		} else {
			r.output <- &Line{LogLine: line, IsError: false, Timestamp: timestamp}
		}
	}

	stopSignal <- true
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

func NewReader(input *os.File, output chan *Line) Reader {
	return &ReaderImpl{bufio.NewScanner(input), output}
}
