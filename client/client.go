package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/markojerkic/svarog/client/reader"
	"github.com/markojerkic/svarog/client/reporter"
	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func readStdin(input chan *reader.Line, done chan bool) {
	readers := []reader.Reader{
		reader.NewReader(os.Stdin, input),
		reader.NewReader(os.Stderr, input),
	}

	numClosed := 0
	closed := make(chan string, len(readers))

	for _, reader := range readers {
		go reader.Run(closed)
	}

	for {
		closedFile := <-closed
		fmt.Println("Closed file: ", closedFile)
		numClosed++
		if closedFile == os.Stdin.Name() || numClosed == len(readers) {
			done <- true
			return
		}
	}

}

func sendLog(reporter reporter.Reporter, input chan *reader.Line) {
	var logLine *reader.Line
	var logLevel rpc.LogLevel
	for {
		logLine = <-input

		// Channel closed, done
		if logLine == nil {
			break
		}

		// fmt.Println(logLine.LogLine)

		if logLine.IsError {
			logLevel = rpc.LogLevel_ERROR
		} else {
			logLevel = rpc.LogLevel_INFO
		}

		reporter.ReportLogLine(&rpc.LogLine{
			Message:   logLine.LogLine,
			Level:     logLevel,
			Timestamp: timestamppb.New(logLine.Timestamp),
			Client:    "client",
		})

	}
}

func main() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	inputQueue := make(chan *reader.Line, 1024*100)
	done := make(chan bool)

	reporter := reporter.NewGrpcReporter(":50051", insecure.NewCredentials())

	go sendLog(reporter, inputQueue)
	go readStdin(inputQueue, done)

	for {
		if !reporter.IsSafeToClose() {
			time.Sleep(1 * time.Second)
		}
	}
}
