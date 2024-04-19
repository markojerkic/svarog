package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DumbLogLine struct {
	Message   string
	Level     rpc.LogLevel
	timestamp *timestamppb.Timestamp
}

func readStdin(input chan DumbLogLine, done chan bool) {
	defer close(input)

	stdErrScanner := bufio.NewScanner(os.Stderr)
	stdInScanner := bufio.NewScanner(os.Stdin)

	var inputLine string

	go func() {
		for stdInScanner.Scan() {

			if stdInScanner.Err() != nil {
				done <- true
				return
			}

			inputLine = stdInScanner.Text()

			input <- DumbLogLine{
				Message:   inputLine,
				Level:     rpc.LogLevel_INFO,
				timestamp: timestamppb.New(time.Now()),
			}
		}
	}()

	go func() {
		for stdErrScanner.Scan() {

			if stdErrScanner.Err() != nil {
				done <- true
				return
			}

			inputLine = stdErrScanner.Text()

			_, err := fmt.Printf("Error: %s\n", inputLine)
			if err != nil {
				done <- true
				return
			}

			input <- DumbLogLine{
				Message:   inputLine,
				Level:     rpc.LogLevel_ERROR,
				timestamp: timestamppb.New(time.Now()),
			}
		}
	}()

	<-done

}

func sendLog(stream rpc.LoggAggregator_LogClient, input chan DumbLogLine) {
	var logLine DumbLogLine
	for {
		logLine = <-input

		err := stream.Send(&rpc.LogLine{
			Message:   logLine.Message,
			Level:     logLine.Level,
			Timestamp: logLine.timestamp,
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

	inputQueue := make(chan DumbLogLine, 1024*100)
	done := make(chan bool)

	go readStdin(inputQueue, done)
	go sendLog(stream, inputQueue)

	<-done
}
