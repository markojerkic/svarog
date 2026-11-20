// Package mcpserver serves Svarog's MCP endpoint as an OAuth protected resource.
package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/archivelink"
	svarogauth "github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/logstore"
)

// instructions tell the model how the tools fit together.
const instructions = `Svarog stores log lines from clients grouped into projects. Each client can run several named instances.
Start with list_projects to find project and client IDs. search_logs finds lines; every line has a link.
get_log_context takes a link (or IDs) and returns the surrounding lines of the same instance.
Results list lines from oldest to newest; pass older_cursor or newer_cursor to get_log_page to move through history.
Old lines may have been moved to archives: list_archives and get_archive_download_link (administrators only).`

const (
	Path                 = "/mcp"
	ResourceMetadataPath = "/.well-known/oauth-protected-resource" + Path
)

type AccessTokens interface {
	VerifyAccessToken(ctx context.Context, token string) (oauth.AccessToken, error)
}

type Users interface {
	GetUserByID(ctx context.Context, id string) (svarogauth.User, error)
	UserHasAccessToProject(ctx context.Context, userID, projectID string) (bool, error)
}

type Projects interface {
	All(ctx context.Context) ([]projects.Project, error)
	ByIDs(ctx context.Context, ids []string) ([]projects.Project, error)
	ClientsForProjects(ctx context.Context, projectIDs []uuid.UUID) ([]projects.ProjectClient, error)
	ClientBelongsToProject(ctx context.Context, projectID, clientID string) (bool, error)
}

type LogReader interface {
	Page(ctx context.Context, query logstore.PageQuery) (logstore.Page, error)
	Around(ctx context.Context, scope logstore.Scope, id int64, limit int, bounds *logstore.TimeRange) (logstore.Page, bool, error)
	Instances(ctx context.Context, scope logstore.Scope) ([]string, error)
}

type Archives interface {
	List(ctx context.Context, scope archive.Scope, query archive.ListQuery) (archive.Page, error)
	Get(ctx context.Context, scope archive.Scope, archiveID uuid.UUID) (archive.Archive, error)
}

type ArchiveLinks interface {
	Sign(claims archivelink.Claims) string
}

type Options struct {
	// Issuer is the OAuth authorization server and the public base URL of
	// links; Resource is this endpoint's URL.
	Issuer       string
	Resource     string
	Tokens       AccessTokens
	Users        Users
	Projects     Projects
	Logs         LogReader
	Archives     Archives
	ArchiveLinks ArchiveLinks
}

// NewHandler serves MCP over stateless Streamable HTTP, so any replica can
// answer any request. Every request needs an access token issued for Resource.
func NewHandler(options Options) http.Handler {
	server := mcp.NewServer(&mcp.Implementation{Name: "svarog", Title: "Svarog", Version: version()}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	registerTools(server, &tools{
		baseURL:      options.Issuer,
		users:        options.Users,
		projects:     options.Projects,
		logs:         options.Logs,
		archives:     options.Archives,
		archiveLinks: options.ArchiveLinks,
	})

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
