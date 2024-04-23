package main

import (
	"context"
	"fmt"
	"os"

	"github.com/markojerkic/svarog/client/reader"
	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc"
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

func sendLog(stream rpc.LoggAggregator_LogClient, input chan *reader.Line) {
	var logLine *reader.Line
	var logLevel rpc.LogLevel
	for {
		logLine = <-input

		if logLine == nil {
			break
		}

		fmt.Println(logLine.LogLine)

		if logLine.IsError {
			logLevel = rpc.LogLevel_ERROR
		} else {
			logLevel = rpc.LogLevel_INFO
		}

		err := stream.Send(&rpc.LogLine{
			Message:   logLine.LogLine,
			Level:     logLevel,
			Timestamp: timestamppb.New(logLine.Timestamp),
            Client:   "client",
		})
		if err != nil {
			return
		}
	}
}

func main() {
	var opts []grpc.DialOption = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(":50051", opts...)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := rpc.NewLoggAggregatorClient(conn)

	stream, err := client.Log(context.Background(), grpc.EmptyCallOption{})
	defer stream.CloseSend()

	if err != nil {
		panic(err)
	}

	inputQueue := make(chan *reader.Line, 1024*100)
	done := make(chan bool)

	go readStdin(inputQueue, done)
	go sendLog(stream, inputQueue)

	<-done
}
