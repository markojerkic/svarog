package reader

import (
	"os"
	"testing"
)

func TestReader(t *testing.T) {
	var reader Reader
	output := make(chan *Line)
	done := make(chan bool)

	// Create a temp file and append lines to it
	f, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	reader = NewReader(f, output)
	for i := 0; i < 100; i++ {
		f.WriteString("line\n")
	}
	go reader.Run(done)

	for i := 0; i < 100; i++ {
		f.WriteString("line\n")
	}
	f.Close()

	numLines := 0
	t.Log("numLines: ", numLines)

	for {
		select {
		case line := <-output:
			if line == nil {
				t.Log("line is nil")
				break
			}
			numLines++
		case <-done:
			if numLines != 200 {
				t.Errorf("expected 100 lines, got %d", numLines)
			}
			return
		}
	}

}
