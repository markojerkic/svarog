package app

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/server/mcpserver"
)

// newOAuthService returns nil when PUBLIC_URL is unset: OAuth needs a stable
// issuer URL, so the MCP endpoint is disabled without one.
func newOAuthService(publicURL string, pool *pgxpool.Pool) (*oauth.Service, error) {
	if publicURL == "" {
		slog.Info("PUBLIC_URL not set; MCP endpoint disabled")
		return nil, nil
	}
	issuer, err := oauth.ParsePublicURL(publicURL)
	if err != nil {
		return nil, err
	}
	slog.Info("MCP endpoint enabled", "url", issuer+mcpserver.Path)
	return oauth.NewService(oauth.Options{
		Pool:     pool,
		Issuer:   issuer,
		Resource: issuer + mcpserver.Path,
		Clients:  oauth.NewMetadataDocumentResolver(),
	}), nil
}
