package serverauth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/jwt/v2"
)

func ParseCredsFile(creds string) (userJwt, seed string, err error) {
	if creds == "" {
		return "", "", errors.New("credentials string is empty")
	}

	userJwt, err = jwt.ParseDecoratedJWT([]byte(creds))
	if err != nil {
		return "", "", fmt.Errorf("failed to parse JWT from credentials: %w", err)
	}

	seed, err = parseRawSeed(creds)
	if err != nil {
		return "", "", err
	}

	return userJwt, seed, nil
}

func parseRawSeed(creds string) (string, error) {
	prefix := "-----BEGIN USER NKEY SEED-----"
	suffix := "------END USER NKEY SEED------"

	start := strings.Index(creds, prefix)
	if start == -1 {
		return "", errors.New("missing NKEY SEED section in credentials")
	}

	content := creds[start+len(prefix):]
	end := strings.Index(content, suffix)
	if end == -1 {
		return "", errors.New("malformed NKEY SEED section in credentials")
	}

	return strings.TrimSpace(content[:end]), nil
}
