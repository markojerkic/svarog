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
	readers := []reader.Reader{
		reader.NewReader(os.Stdin, output),
		reader.NewReader(os.Stderr, output),
	}

	waitGroup := &sync.WaitGroup{}

	for _, reader := range readers {
		waitGroup.Add(1)
		go reader.Run(ctx, waitGroup)
	}

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

	processedLines := make(chan *commontypes.LogLineDto, 1024*1024)
	natsClient := natsclient.NewNatsClient(config, processedLines)

	go natsClient.Run(context.Background())
	readStdin(context.Background(), processedLines)

}
