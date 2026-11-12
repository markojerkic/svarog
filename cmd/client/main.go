package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/markojerkic/svarog/cmd/client/reader"
	"github.com/markojerkic/svarog/internal/lib/svarogclient"
)

func getConnString() (string, error) {
	if len(os.Args) > 2 {
		return "", errors.New("usage: svarog-client [connection URL]")
	}
	var connString string
	if len(os.Args) == 2 {
		connString = os.Args[1]
	} else {
		connString = os.Getenv("SVAROG_CONN_STRING")
	}
	if connString == "" {
		return "", errors.New("provide a connection URL as the first argument or set SVAROG_CONN_STRING")
	}
	return connString, nil
}

func run() error {
	connString, err := getConnString()
	if err != nil {
		return err
	}
	client, err := svarogclient.New(svarogclient.Options{ConnectionString: connString})
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	readDone := make(chan error, 1)
	go func() {
		readDone <- reader.NewReader(os.Stdin, client.Enqueue).Run(ctx)
	}()
	var readErr error
	readStopped := false
	select {
	case readErr = <-readDone:
		readStopped = true
	case <-ctx.Done():
	}
	signaled := ctx.Err() != nil
	stop()
	if signaled {
		// Best effort: Close does not interrupt a read already blocked on an
		// inherited blocking descriptor.
		_ = os.Stdin.Close()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	shutdownErr := client.Shutdown(shutdownCtx)
	if !readStopped {
		// Do not wait for a reader blocked in read(2); process exit ends it.
		select {
		case readErr = <-readDone:
		default:
		}
	}
	if signaled && (errors.Is(readErr, context.Canceled) || errors.Is(readErr, svarogclient.ErrClosed)) {
		// Publisher failures still surface through shutdownErr.
		readErr = nil
	}
	return errors.Join(readErr, shutdownErr)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
