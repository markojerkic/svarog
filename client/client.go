package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

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
		slog.Debug("Closed file", slog.String("file", closedFile))
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

		fmt.Println(logLine.LogLine)

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
	debugLogEnabled := flag.Bool("debug", false, "Enable debug mode")
	serverAddr := flag.String("server", ":50051", "Server address")
	flag.Parse()

	opts := &slog.HandlerOptions{}

	if *debugLogEnabled {
		opts.Level = slog.LevelDebug
	} else {
		opts.Level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	inputQueue := make(chan *reader.Line, 1024*100)
	done := make(chan bool)

	reporter := reporter.NewGrpcReporter(*serverAddr, insecure.NewCredentials())

	go sendLog(reporter, inputQueue)
	go readStdin(inputQueue, done)

	<-done
}
