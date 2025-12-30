package main

import (
	"context"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/cmd/client/config"
	natsclient "github.com/markojerkic/svarog/cmd/client/nats-client"
	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/commontypes"
)

func readStdin(ctx context.Context, output chan *commontypes.LogLineDto) {
	r := reader.NewReader(os.Stdin, output)

	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	go r.Run(ctx, waitGroup)

	waitGroup.Wait()
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

	if config.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.ErrorLevel)
	}

	log.Debug("Parsed config", "config", config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processedLines := make(chan *commontypes.LogLineDto, 1024*1024)
	natsClient := natsclient.NewNatsClient(config, processedLines)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		natsClient.Run(ctx)
	}()

	readStdin(ctx, processedLines)
	close(processedLines)
	cancel() // Signal NATS client to stop (in case it's still connecting)
	wg.Wait()
}
