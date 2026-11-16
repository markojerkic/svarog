package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type logEntry struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func run() error {
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
		if _, err := fmt.Fprintf(os.Stderr, "timestamp=%s message=%q\n",
			entry.Timestamp.Format(time.RFC3339Nano), entry.Message); err != nil {
			return fmt.Errorf("write log entry: %w", err)
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
