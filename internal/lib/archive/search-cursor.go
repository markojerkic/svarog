package archive

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

// Derive a signing key for this exact search. Canonicalize the fixed-width
// ID + SHA-256 pairs independently of the newest-first ZIP scan order.
func searchCursorKey(secret string, scope Scope, q SearchQuery, candidates []sqlc.ListSearchArchivesRow) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("svarog/archive-search/v1\x00"))
	mac.Write(scope.ProjectID[:])
	mac.Write(scope.ClientID[:])
	for _, stamp := range []time.Time{q.From, q.To} {
		mac.Write([]byte(stamp.UTC().Format(time.RFC3339Nano) + "\x00"))
	}
	candidates = slices.Clone(candidates)
	slices.SortFunc(candidates, func(a, b sqlc.ListSearchArchivesRow) int { return bytes.Compare(a.ID[:], b.ID[:]) })
	mac.Write(binary.BigEndian.AppendUint32(nil, uint32(len(candidates))))
	for _, row := range candidates {
		mac.Write(row.ID[:])
		mac.Write(row.Sha256)
	}
	mac.Write([]byte(q.Text))
	return mac.Sum(nil)
}

// The payload is just the compareHits tuple. Seconds and nanoseconds are
// separate so even dates outside UnixNano's range round-trip exactly.
const positionHeader = 8 + 4 + 16 + 4 // seconds, nanos, UUID, physical line

func encodeSearchCursor(key []byte, hit SearchHit) string {
	payload := binary.BigEndian.AppendUint64(nil, uint64(hit.OccurredAt.Unix()))
	payload = binary.BigEndian.AppendUint32(payload, uint32(hit.OccurredAt.Nanosecond()))
	payload = append(payload, hit.ArchiveID[:]...)
	payload = binary.BigEndian.AppendUint32(payload, uint32(hit.Line))
	payload = append(payload, hit.Entry...)
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(payload))
}

func decodeSearchCursor(key []byte, token string) (*SearchHit, error) {
	// A writer-generated entry name includes at most a percent-encoded 1 KiB instance.
	if len(token) > 8192 {
		return nil, ErrSearchCursor
	}
	raw, err := base64.RawURLEncoding.Strict().DecodeString(token)
	if err != nil || len(raw) <= positionHeader+sha256.Size || len(token) != base64.RawURLEncoding.EncodedLen(len(raw)) {
		return nil, ErrSearchCursor
	}
	payload := raw[:len(raw)-sha256.Size]
	mac := hmac.New(sha256.New, key)
	mac.Write(payload)
	if !hmac.Equal(mac.Sum(nil), raw[len(payload):]) {
		return nil, ErrSearchCursor
	}
	return &SearchHit{
		OccurredAt: time.Unix(int64(binary.BigEndian.Uint64(payload)), int64(binary.BigEndian.Uint32(payload[8:]))).UTC(),
		ArchiveID:  uuid.UUID(payload[12:28]),
		Line:       int(binary.BigEndian.Uint32(payload[28:])),
		Entry:      string(payload[positionHeader:]),
	}, nil
}
