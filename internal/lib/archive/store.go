package archive

import (
	"context"
	"io"
)

// Store persists archive content independently of policies, ZIP generation and
// PostgreSQL. Keys are slash-separated relative object names, not public paths.
type Store interface {
	// Backend is the stable storage_backend value persisted with each archive.
	Backend() string
	// Put calls write synchronously, at most once, with a streaming writer.
	// Success means write succeeded and all bytes are durably stored and available
	// through Open; metadata describes those exact bytes. An error may leave
	// content behind: callers must confirm database rollback before Delete.
	Put(ctx context.Context, key string, write func(io.Writer) error) (StoredObject, error)
	// Open rejects missing content or a size mismatch. The caller closes the
	// stream; seeking and Range support are not required.
	Open(ctx context.Context, key string, sizeBytes int64) (io.ReadCloser, error)
	// Delete removes both completed content and any staging artifacts for key.
	// It is idempotent for missing content. Callers must hold exclusive build
	// ownership and confirm the archive is neither ready nor possibly committed.
	Delete(ctx context.Context, key string) error
}

type StoredObject struct {
	SizeBytes int64
	SHA256    [32]byte
}
