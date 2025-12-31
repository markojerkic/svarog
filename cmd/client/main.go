package main

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/cmd/client/config"
	natsclient "github.com/markojerkic/svarog/cmd/client/nats-client"
	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/rpc"
)

func readStdin(output chan *rpc.LogLine) {
	r := reader.NewReader(os.Stdin, output)

	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	go r.Run(context.Background(), waitGroup)

	waitGroup.Wait()
}

func setupLogger(debug bool) {
	util.SetupLogger(util.LoggerOptions{Debug: debug})
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Expected 1 argument", "len", len(os.Args)-1, "args", os.Args)
	}
	connString := os.Args[1]

	config, err := config.NewClientConfig(connString)
	if err != nil {
		log.Fatal("Failed to parse connection string", "err", err)
	}

	setupLogger(config.Debug)

	slog.Debug("Parsed config", "config", config)

	processedLines := make(chan *rpc.LogLine, 1024*1024)
	natsClient := natsclient.NewNatsClient(config, processedLines)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		natsClient.Run()
	}()

	readStdin(processedLines)
	close(processedLines) // Signal NATS client to drain and exit
	wg.Wait()
}
