// Package archivelink signs short-lived archive download links. A link names
// its user, client and archive; the download still checks that the user has
// access, so revoking access also invalidates links already issued.
package archivelink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	// Path is followed by the token.
	Path = "/archives/download/"
	TTL  = 15 * time.Minute

	version     = 1
	payloadSize = 1 + 4*16 + 8
)

var ErrInvalid = errors.New("invalid or expired archive download link")

type Claims struct {
	UserID    uuid.UUID
	ProjectID uuid.UUID
	ClientID  uuid.UUID
	ArchiveID uuid.UUID
	ExpiresAt time.Time
}

type Signer struct {
	key []byte
}

// NewSigner derives a key used only for download links from the server secret.
func NewSigner(secret string) *Signer {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("svarog/archive-download/v1"))
	return &Signer{key: mac.Sum(nil)}
}

// Sign returns a URL-safe token for the claims.
func (s *Signer) Sign(claims Claims) string {
	payload := make([]byte, 0, payloadSize+sha256.Size)
	payload = append(payload, version)
	for _, id := range []uuid.UUID{claims.UserID, claims.ProjectID, claims.ClientID, claims.ArchiveID} {
		payload = append(payload, id[:]...)
	}
	payload = binary.BigEndian.AppendUint64(payload, uint64(claims.ExpiresAt.Unix()))
	return base64.RawURLEncoding.EncodeToString(append(payload, s.mac(payload)...))
}

// Verify returns the claims of an authentic token that has not expired.
func (s *Signer) Verify(token string, now time.Time) (Claims, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != payloadSize+sha256.Size {
		return Claims{}, ErrInvalid
	}
	payload, signature := raw[:payloadSize], raw[payloadSize:]
	if !hmac.Equal(signature, s.mac(payload)) || payload[0] != version {
		return Claims{}, ErrInvalid
	}
	ids := make([]uuid.UUID, 4)
	for i := range ids {
		copy(ids[i][:], payload[1+16*i:1+16*(i+1)])
	}
	expiresAt := time.Unix(int64(binary.BigEndian.Uint64(payload[1+4*16:])), 0)
	if !now.Before(expiresAt) {
		return Claims{}, ErrInvalid
	}
	return Claims{UserID: ids[0], ProjectID: ids[1], ClientID: ids[2], ArchiveID: ids[3], ExpiresAt: expiresAt}, nil
}

func (s *Signer) mac(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write(payload)
	return mac.Sum(nil)
}
