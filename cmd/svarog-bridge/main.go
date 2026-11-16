package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/markojerkic/svarog/internal/lib/svarogclient"
)

type logEntry struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func run() (result error) {
	connectionString := os.Getenv("SVAROG_CONN_STRING")
	if connectionString == "" {
		return errors.New("SVAROG_CONN_STRING is required")
	}
	client, err := svarogclient.New(svarogclient.Options{ConnectionString: connectionString})
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		result = errors.Join(result, client.Shutdown(ctx))
	}()

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)

	for scanner.Scan() {
		var entry logEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return fmt.Errorf("decode log entry: %w", err)
		}
		if entry.Timestamp.IsZero() {
			return errors.New("log entry timestamp is required")
		}
		if err := client.Enqueue(svarogclient.Entry{
			Message: entry.Message, Timestamp: entry.Timestamp,
		}); err != nil {
			return fmt.Errorf("enqueue log entry: %w", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read log entry: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
