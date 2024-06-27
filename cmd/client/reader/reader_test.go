package reader

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reader", func() {

	It("should implement the Reader interface", func() {
		var _ Reader = (*ReaderImpl)(nil) // Ensure ReaderImpl implements the Reader interface
	})

	It("should read lines from a file", func() {

		input, err := os.CreateTemp("", "testfile.txt")
		defer os.Remove(input.Name())

		Expect(err).To(BeNil())

		input.WriteString("test\nline\n")
		input.Close()

		output := make(chan *Line, 10)

		input, err = os.Open(input.Name())

		Expect(err).To(BeNil())

		r := NewReader(input, output)
		stopSignal := make(chan string)

		go r.Run(stopSignal)

		expectedLines := []string{"test", "line"}

		receivedLines := make([]string, 0, 2)

		// Wait for Run to finish
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

		Expect(len(receivedLines)).To(Equal(len(expectedLines)))

		for i, line := range expectedLines {
			Expect(receivedLines[i]).To(Equal(line))
		}

	})

})
