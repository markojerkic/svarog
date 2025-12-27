package main

import (
	"context"
	"os"
	"sync"

	"github.com/charmbracelet/log"
	grpcclient "github.com/markojerkic/svarog/cmd/client/grpc-client"
	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/cmd/client/retry"
	"github.com/markojerkic/svarog/internal/lib/backlog"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/spf13/cobra"
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
	serverName      string
}

func getOsEnv() Env {
	env := Env{
		debugLogEnabled: os.Getenv("SVAROG_DEBUG_ENABLED") == "true",
		serverAddr:      os.Getenv("SVAROG_SERVER_ADDR"),
		clientId:        os.Getenv("SVAROG_CLIENT_ID"),
		certificatePath: os.Getenv("SVAROG_CERTIFICATE_PATH"),
		serverName:      os.Getenv("SVAROG_SERVER_NAME"),
	}

	return env
}

func configureLogging(env Env) {
	log.SetReportCaller(true)

	if env.debugLogEnabled {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.ErrorLevel)
	}
}

func run(env Env) {
	configureLogging(env)

	processedLines := make(chan *rpc.LogLine, 1024*1024)

	backlog := backlog.NewBacklog[*rpc.LogLine](1024 * 1024)
	retryService := retry.NewRetry(backlog.GetLogs(), 5)

	caCertPath, certPath := grpcclient.UnzipCredentials(env.certificatePath)
	defer func() {
		os.Remove(caCertPath)
		os.Remove(certPath)
	}()
	tlsConfig := grpcclient.BuildCredentials(caCertPath, certPath, env.serverName)

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

func main() {
	var debugLogEnabled bool
	var serverAddr string
	var clientId string
	var certificatePath string
	var serverName string

	rootCmd := &cobra.Command{
		Use:   "svarog-client",
		Short: "Svarog log client",
		Long:  "Client for streaming logs to Svarog server",
		Run: func(cmd *cobra.Command, args []string) {
			osEnv := getOsEnv()

			if osEnv.serverAddr != "" && osEnv.clientId != "" {
				run(osEnv)
				return
			}

			if serverAddr == "" {
				log.Fatal("Server address must be provided")
			}

			if clientId == "" {
				log.Fatal("Client ID must be provided")
			}

			if certificatePath == "" {
				log.Fatal("Certificate path must be provided")
			}

			if serverName == "" {
				serverName = "localhost"
			}

			env := Env{
				debugLogEnabled: debugLogEnabled,
				serverAddr:      serverAddr,
				clientId:        clientId,
				certificatePath: certificatePath,
				serverName:      serverName,
			}

			run(env)
		},
	}

	rootCmd.Flags().BoolVar(&debugLogEnabled, "debug", false, "Enable debug mode")
	rootCmd.Flags().StringVar(&serverAddr, "server", "", "Server address")
	rootCmd.Flags().StringVar(&clientId, "client-id", "", "Client ID")
	rootCmd.Flags().StringVar(&certificatePath, "cert", "", "Path to certificates zip file")
	rootCmd.Flags().StringVar(&serverName, "server-name", "", "Server name for TLS verification (defaults to localhost)")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
