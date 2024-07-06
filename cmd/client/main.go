package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"sync"

	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/cmd/client/retry"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"google.golang.org/grpc/credentials/insecure"
)

func readStdin(clientId string, output chan *rpc.LogLine) {
	readers := []reader.Reader{
		reader.NewReader(os.Stdin, clientId, output),
		reader.NewReader(os.Stderr, clientId, output),
	}

	waitGroup := &sync.WaitGroup{}

	for _, reader := range readers {
		waitGroup.Add(1)
		go reader.Run(context.Background(), waitGroup)
	}

	waitGroup.Wait()
}

type Env struct {
	debugLogEnabled bool
	serverAddr      string
	clientId        string
}

func getEnv() Env {
	debugLogEnabled := flag.Bool("debug", false, "Enable debug mode")
	serverAddr := flag.String("server", ":50051", "Server address")
	clientId := flag.String("clientId", "client", "Client ID")
	flag.Parse()

	return Env{
		debugLogEnabled: *debugLogEnabled,
		serverAddr:      *serverAddr,
		clientId:        *clientId,
	}
}

func configureLogging(env Env) {
	opts := &slog.HandlerOptions{}

	if env.debugLogEnabled {
		opts.Level = slog.LevelDebug
	} else {
		opts.Level = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func main() {
	env := getEnv()
	configureLogging(env)

	processedLines := make(chan *rpc.LogLine, 1024*100)

	backlog := db.NewBacklog[*rpc.LogLine](1024 * 1024)
	retryService := retry.NewRetry(backlog.GetLogs(), 5)
	grpcClient := NewClient(env.serverAddr, insecure.NewCredentials())

	go retryService.Run(context.Background(), func(ll []*rpc.LogLine) {
		for _, line := range ll {
			processedLines <- line
		}
	})
	go grpcClient.Run(context.Background(), processedLines, func(ll *rpc.LogLine) {
		backlog.AddToBacklog(ll)
	})

	readStdin(env.clientId, processedLines)
}
