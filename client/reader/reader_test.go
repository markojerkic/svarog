package reader

import (
	"os"
	"testing"
	"time"
)

func TestReaderImplementation(t *testing.T) {
	var _ Reader = (*ReaderImpl)(nil) // Ensure ReaderImpl implements the Reader interface
}

func TestReaderRun(t *testing.T) {
	input, err := os.CreateTemp("", "testfile.txt")
	defer os.Remove(input.Name())

	if err != nil {
		t.Fatal(err)
	}

	input.WriteString("test\nline\n")
	input.Close()

	output := make(chan *Line, 10)

	input, err = os.Open(input.Name())

	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}

	r := NewReader(input, output)
	stopSignal := make(chan string)

	go r.Run(stopSignal)

	expectedLines := []string{"test", "line"}

	receivedLines := make([]string, 0, 2)

	// Wait for Run to finish
	t.Log("Waiting for Run to finish")
	<-stopSignal

loop:
	for {
		select {
		case line := <-output:
			if line == nil {
				break
			}

			receivedLines = append(receivedLines, line.LogLine)
		case <-time.After(1 * time.Second):
			break loop
		}
	}

	if len(receivedLines) != len(expectedLines) {
		t.Errorf("Expected %d lines, got %d", len(expectedLines), len(receivedLines))
	}

	for i, line := range expectedLines {
		if receivedLines[i] != line {
			t.Errorf("Expected line '%s', got '%s'", line, receivedLines[i])
		}
	}

}
