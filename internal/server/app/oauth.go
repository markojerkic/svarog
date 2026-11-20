package app

import (
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/markojerkic/svarog/internal/lib/archivelink"
	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/server/mcpserver"
)

// newMCPAuth returns nil services when PUBLIC_URL is unset: OAuth needs a
// stable issuer URL, so the MCP endpoint is disabled without one.
func newMCPAuth(publicURL, sessionSecret string, pool *pgxpool.Pool) (*oauth.Service, *archivelink.Signer, error) {
	if publicURL == "" {
		slog.Info("PUBLIC_URL not set; MCP endpoint disabled")
		return nil, nil, nil
	}
	issuer, err := oauth.ParsePublicURL(publicURL)
	if err != nil {
		return nil, nil, err
	}
	if sessionSecret == "" {
		return nil, nil, errors.New("SESSION_SECRET is required to sign archive download links")
	}
	slog.Info("MCP endpoint enabled", "url", issuer+mcpserver.Path)
	service := oauth.NewService(oauth.Options{
		Pool:     pool,
		Issuer:   issuer,
		Resource: issuer + mcpserver.Path,
		Clients:  oauth.NewMetadataDocumentResolver(),
	})
	return service, archivelink.NewSigner(sessionSecret), nil
}
