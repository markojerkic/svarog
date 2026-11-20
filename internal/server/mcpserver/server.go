// Package mcpserver serves Svarog's MCP endpoint as an OAuth protected resource.
package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	svarogauth "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/lib/projects"
)

const (
	Path                 = "/mcp"
	ResourceMetadataPath = "/.well-known/oauth-protected-resource" + Path
)

type AccessTokens interface {
	VerifyAccessToken(ctx context.Context, token string) (oauth.AccessToken, error)
}

type Users interface {
	GetUserByID(ctx context.Context, id string) (svarogauth.User, error)
}

type Projects interface {
	All(ctx context.Context) ([]projects.Project, error)
	ByIDs(ctx context.Context, ids []string) ([]projects.Project, error)
}

type Options struct {
	// Issuer is the OAuth authorization server, and Resource is this endpoint's URL.
	Issuer   string
	Resource string
	Tokens   AccessTokens
	Users    Users
	Projects Projects
}

// NewHandler serves MCP over stateless Streamable HTTP, so any replica can
// answer any request. Every request needs an access token issued for Resource.
func NewHandler(options Options) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "svarog", Title: "Svarog", Version: version()}, nil)
	registerTools(server, &tools{users: options.Users, projects: options.Projects})

	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless: true,
			// Bearer tokens already defeat DNS rebinding, and a reverse proxy on
			// the loopback interface forwards the public Host header.
			DisableLocalhostProtection: true,
		},
	)
	bearer := auth.RequireBearerToken(verifyAccessToken(options.Tokens), &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: options.Issuer + ResourceMetadataPath,
		Scopes:              []string{oauth.ScopeLogsRead},
	})
	return allowBrowserClients(bearer(streamable))
}

// allowBrowserClients lets MCP clients running in a browser call the endpoint
// from any origin. The endpoint ignores cookies and requires a bearer token,
// which a cross-site page cannot obtain, so CORS grants it nothing more.
// Preflight requests carry no token and are answered before authentication.
func allowBrowserClients(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Expose-Headers", "WWW-Authenticate, Mcp-Session-Id, Mcp-Protocol-Version")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			header.Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
			// The Authorization header is never covered by a wildcard.
			if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
				header.Set("Access-Control-Allow-Headers", requested)
			}
			header.Add("Vary", "Access-Control-Request-Headers")
			header.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// NewResourceMetadataHandler serves the RFC 9728 metadata that points MCP
// clients to the authorization server.
func NewResourceMetadataHandler(issuer, resource string) http.Handler {
	return auth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   []string{issuer},
		ScopesSupported:        []string{oauth.ScopeLogsRead},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Svarog",
	})
}

func verifyAccessToken(tokens AccessTokens) auth.TokenVerifier {
	return func(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		access, err := tokens.VerifyAccessToken(ctx, token)
		if errors.Is(err, oauth.ErrInvalidAccessToken) {
			return nil, auth.ErrInvalidToken
		}
		if err != nil {
			// The middleware writes the error text into the response.
			slog.ErrorContext(ctx, "MCP access token verification failed", "error", err)
			return nil, errors.New("access token verification failed")
		}
		return &auth.TokenInfo{
			Scopes:     access.Scopes,
			Expiration: access.ExpiresAt,
			UserID:     access.UserID.String(),
		}, nil
	}
}

func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "devel"
}
