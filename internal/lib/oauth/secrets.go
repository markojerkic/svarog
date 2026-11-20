package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
)

// Prefixes identify leaked secrets and keep each kind out of the others' lookups.
const (
	codePrefix         = "svarog_oac_"
	accessTokenPrefix  = "svarog_oat_"
	refreshTokenPrefix = "svarog_ort_"
)

func newSecret(prefix string) string {
	return prefix + rand.Text()
}

// Refresh tokens name their grant, so any earlier refresh token of a grant is
// recognised as reuse after rotation, not only the previous one.
func newRefreshToken(grantID uuid.UUID) string {
	return refreshTokenPrefix + hex.EncodeToString(grantID[:]) + "_" + rand.Text()
}

func refreshTokenGrant(token string) (uuid.UUID, bool) {
	rest, ok := strings.CutPrefix(token, refreshTokenPrefix)
	if !ok {
		return uuid.Nil, false
	}
	encodedID, secret, ok := strings.Cut(rest, "_")
	if !ok || secret == "" || len(encodedID) != hex.EncodedLen(len(uuid.Nil)) {
		return uuid.Nil, false
	}
	var id uuid.UUID
	if _, err := hex.Decode(id[:], []byte(encodedID)); err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// The database stores only SHA-256 hashes of codes and tokens.
func hashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// validCodeChallenge accepts an unpadded base64url SHA-256 digest (RFC 7636).
func validCodeChallenge(challenge string) bool {
	if len(challenge) != 43 {
		return false
	}
	for _, r := range challenge {
		if !isBase64URL(r) {
			return false
		}
	}
	return true
}

func validCodeVerifier(verifier string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	for _, r := range verifier {
		if !isBase64URL(r) && r != '.' && r != '~' {
			return false
		}
	}
	return true
}

func verifyCodeVerifier(verifier, challenge string) bool {
	if !validCodeVerifier(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}

func isBase64URL(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
}
