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

func getInstanceId() string {
	instanceId := os.Getenv("SVAROG_INSTANCE_ID")
	if instanceId != "" {
		return instanceId
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Warn("Failed to get hostname, using 'unknown'", "error", err)
		return "unknown"
	}

	return hostname
}

func readStdin(output chan *rpc.LogLine, instanceId string) {
	r := reader.NewReader(os.Stdin, output, instanceId)

	waitGroup := &sync.WaitGroup{}
	waitGroup.Add(1)
	go r.Run(context.Background(), waitGroup)

	waitGroup.Wait()
}

func setupLogger(debug bool) {
	util.SetupLogger(util.LoggerOptions{Debug: debug})
}

func getConnString() string {
	var connString string
	if len(os.Args) == 2 {
		connString = os.Args[1]
	} else if len(os.Args) == 1 {
		connString = os.Getenv("SVAROG_CONN_STRING")
	}

	if connString == "" {
		log.Fatal("Connection string not provided", "add as the first argument or set env var SVAROG_CONN_STRING")
	}

	return connString
}

func main() {
	connString := getConnString()

	config, err := config.NewClientConfig(connString)
	if err != nil {
		log.Fatal("Failed to parse connection string", "err", err)
	}

	setupLogger(config.Debug)

	instanceId := getInstanceId()
	slog.Debug("Instance ID", "id", instanceId)

	processedLines := make(chan *rpc.LogLine, 1024*1024)
	natsClient := natsclient.NewNatsClient(config, processedLines)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		natsClient.Run()
	}()

	readStdin(processedLines, instanceId)
	close(processedLines) // Signal NATS client to drain and exit
	wg.Wait()
}
