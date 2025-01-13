package main

import (
	"context"
	"flag"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	grpcclient "github.com/markojerkic/svarog/cmd/client/grpc-client"
	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/cmd/client/retry"
	"github.com/markojerkic/svarog/internal/lib/backlog"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"google.golang.org/grpc/credentials"
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
	certificatePath string
}

func getOsEnv() Env {
	env := Env{
		debugLogEnabled: os.Getenv("SVAROG_DEBUG_ENABLED") == "true",
		serverAddr:      os.Getenv("SVAROG_SERVER_ADDR"),
		clientId:        os.Getenv("SVAROG_CLIENT_ID"),
		certificatePath: os.Getenv("SVAROG_CERTIFICATE_PATH"),
	}

	return env
}

func getEnv() Env {
	osEnv := getOsEnv()

	if osEnv.serverAddr != "" && osEnv.clientId != "" {
		return osEnv
	}

	debugLogEnabled := flag.Bool("SVAROG_DEBUG_ENABLED", osEnv.debugLogEnabled, "Enable debug mode")
	serverAddr := flag.String("SVAROG_SERVER_ADDR", osEnv.serverAddr, "Server address")
	clientId := flag.String("SVAROG_CLIENT_ID", osEnv.clientId, "Client ID")
	certificatePath := flag.String("SVAROG_CERTIFICATE_PATH", osEnv.certificatePath, "Path to certificates zip file")
	flag.Parse()

	if serverAddr == nil || *serverAddr == "" {
		log.Fatal("Server address must be provided")
	}

	if clientId == nil || *clientId == "" {
		log.Fatal("Client ID must be provided")
	}

	return Env{
		debugLogEnabled: *debugLogEnabled,
		serverAddr:      *serverAddr,
		clientId:        *clientId,
		certificatePath: *certificatePath,
	}
}

func configureLogging(env Env) {
	log.SetReportCaller(true)

	if env.debugLogEnabled {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.ErrorLevel)
	}

}

func main() {
	env := getEnv()
	configureLogging(env)

	processedLines := make(chan *rpc.LogLine, 1024*1024)

	backlog := backlog.NewBacklog[*rpc.LogLine](1024 * 1024)
	retryService := retry.NewRetry(backlog.GetLogs(), 5)

	caCertPath, certPath := grpcclient.UnzipCredentials(env.certificatePath)
	defer func() {
		os.Remove(caCertPath)
		os.Remove(certPath)
	}()
	tlsConfig := grpcclient.BuildCredentials(caCertPath, certPath)

	grpcClient := grpcclient.NewClient(env.serverAddr, credentials.NewTLS(tlsConfig))

	go retryService.Run(context.Background(), func(logLines []*rpc.LogLine) {
		err := grpcClient.BatchSend(logLines)
		if err != nil {
			log.Info("Failed to retry batch insert. Returning to backlog")
			backlog.AddAllToBacklog(logLines)
		}
	})
	go grpcClient.Run(context.Background(), processedLines, backlog.AddToBacklog)

	readStdin(env.clientId, processedLines)
}
