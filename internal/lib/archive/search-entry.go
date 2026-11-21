package archive

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/markojerkic/svarog/internal/lib/logevent"
)

// The writer encodes instance names reversibly, including empty names, slashes
// and case-folding collisions. The header must match this exact instance.
func entryInstance(name string) (string, error) {
	dir, file, ok := strings.Cut(name, "/")
	if !ok || dir == "" || strings.Contains(file, "/") || !strings.HasPrefix(file, "logs_") || !strings.HasSuffix(file, ".log") {
		return "", ErrArchiveUnavailable
	}
	if encoded, ok := strings.CutPrefix(dir, "="); ok {
		b, err := hex.DecodeString(encoded)
		return string(b), err
	}
	return url.PathUnescape(dir)
}

func scanLogEntry(ctx context.Context, entry *zip.File, id uuid.UUID, visit func(SearchHit) error) (int64, error) {
	instance, err := entryInstance(entry.Name)
	if err != nil || logevent.ValidateInstanceID(instance) != nil {
		return 0, ErrArchiveUnavailable
	}
	rc, err := entry.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()
	r := bufio.NewReader(searchReader{ctx, rc})
	var message strings.Builder
	var count int64
	hit := SearchHit{ArchiveID: id, Entry: entry.Name, Instance: instance}
	flush := func() error {
		if hit.Line == 0 {
			return nil
		}
		hit.Message = strings.TrimSuffix(message.String(), "\n") // remove only the writer's delimiter
		message.Reset()
		count++
		return visit(hit)
	}
	suffix := []byte(" " + instance + "] ")
	for line := 1; ; line++ {
		first, err := r.Peek(1)
		if err == io.EOF {
			err := flush()
			return count, err
		}
		if err != nil {
			return 0, err
		}
		if first[0] == '[' {
			// Peek across physical lines: an instance itself can contain a newline.
			prefix, _ := r.Peek(40 + len(suffix))
			i := bytes.IndexByte(prefix, ' ')
			if i > 1 && bytes.HasPrefix(prefix[i:], suffix) {
				if stamp, err := time.Parse(time.RFC3339Nano, string(prefix[1:i])); err == nil {
					if err := flush(); err != nil {
						return 0, err
					}
					hit.OccurredAt, hit.Line = stamp, line
					_, _ = r.Discard(i + len(suffix))
					line += strings.Count(instance, "\n")
				}
			}
		}
		if hit.Line == 0 {
			return 0, fmt.Errorf("missing first record header: %w", ErrArchiveUnavailable)
		}
		for {
			part, err := r.ReadSlice('\n')
			if message.Len()+len(part) > logevent.MaxMessageBytes+1 {
				return 0, ErrSearchLimit
			}
			message.Write(part)
			if err == nil {
				break
			}
			if err != bufio.ErrBufferFull {
				if err == io.EOF {
					return 0, io.ErrUnexpectedEOF
				}
				return 0, err
			}
		}
	}
}
