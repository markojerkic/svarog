package main

import (
	"context"
	"fmt"
	"time"

	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	println("Hello, World!")

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

	if err != nil {
		panic(err)
	}

	logCount := 0

	for {
		err = stream.Send(&rpc.LogLine{
			Message:   fmt.Sprintf("Log message %d", logCount),
			Level:     *rpc.LogLevel_INFO.Enum(),
			Timestamp: timestamppb.New(time.Now()),
		})

		if err != nil {
			panic(err)
		}

		// set a delay to simulate a real-world scenario
		// make the delay be random 100 to 300 ms
		time.Sleep(time.Duration(10+logCount%200) * time.Millisecond)

		logCount++

		if logCount == 100 {
			break
		}
	}
}
