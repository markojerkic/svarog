// Package oauth is Svarog's OAuth 2.1 authorization server for MCP clients.
// It supports the authorization code grant with PKCE (S256) and rotating
// refresh tokens. Clients identify themselves with OAuth Client ID Metadata
// Documents, so every client is public and nothing is registered in advance.
package oauth

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

const (
	// ScopeLogsRead is the only scope; it grants the user's own read access.
	ScopeLogsRead = "logs:read"

	MetadataPath      = "/.well-known/oauth-authorization-server"
	AuthorizationPath = "/oauth/authorize"
	TokenPath         = "/oauth/token"

	codeTTL         = time.Minute
	accessTokenTTL  = time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
)

// OAuth error codes from RFC 6749, RFC 8707 and the authorization response.
const (
	ErrorInvalidRequest          = "invalid_request"
	ErrorInvalidClient           = "invalid_client"
	ErrorInvalidGrant            = "invalid_grant"
	ErrorInvalidTarget           = "invalid_target"
	ErrorUnsupportedGrantType    = "unsupported_grant_type"
	ErrorUnsupportedResponseType = "unsupported_response_type"
	ErrorAccessDenied            = "access_denied"
	ErrorTemporarilyUnavailable  = "temporarily_unavailable"
)

type Options struct {
	Pool *pgxpool.Pool
	// Issuer is the canonical public origin returned by ParsePublicURL.
	Issuer string
	// Resource is the canonical URL of the protected MCP endpoint.
	Resource string
	Clients  ClientResolver
}

type Service struct {
	queries  *sqlc.Queries
	issuer   string
	resource string
	clients  ClientResolver
}

func NewService(options Options) *Service {
	return &Service{
		queries:  sqlc.New(options.Pool),
		issuer:   options.Issuer,
		resource: options.Resource,
		clients:  options.Clients,
	}
}

func (s *Service) Issuer() string   { return s.issuer }
func (s *Service) Resource() string { return s.resource }

// ServerMetadata is the RFC 8414 authorization server metadata document.
type ServerMetadata struct {
	Issuer                                     string   `json:"issuer"`
	AuthorizationEndpoint                      string   `json:"authorization_endpoint"`
	TokenEndpoint                              string   `json:"token_endpoint"`
	ScopesSupported                            []string `json:"scopes_supported"`
	ResponseTypesSupported                     []string `json:"response_types_supported"`
	ResponseModesSupported                     []string `json:"response_modes_supported"`
	GrantTypesSupported                        []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported          []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported              []string `json:"code_challenge_methods_supported"`
	ClientIDMetadataDocumentSupported          bool     `json:"client_id_metadata_document_supported"`
	AuthorizationResponseIssParameterSupported bool     `json:"authorization_response_iss_parameter_supported"`
}

func (s *Service) Metadata() ServerMetadata {
	return ServerMetadata{
		Issuer:                                     s.issuer,
		AuthorizationEndpoint:                      s.issuer + AuthorizationPath,
		TokenEndpoint:                              s.issuer + TokenPath,
		ScopesSupported:                            []string{ScopeLogsRead},
		ResponseTypesSupported:                     []string{"code"},
		ResponseModesSupported:                     []string{"query"},
		GrantTypesSupported:                        []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported:          []string{"none"},
		CodeChallengeMethodsSupported:              []string{"S256"},
		ClientIDMetadataDocumentSupported:          true,
		AuthorizationResponseIssParameterSupported: true,
	}
}

// ParsePublicURL returns the canonical origin used as the OAuth issuer.
// Svarog serves its routes from the root, so the URL cannot have a path. OAuth
// requires HTTPS except for a loopback development server.
func ParsePublicURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Opaque != "" ||
		u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return "", errors.New("PUBLIC_URL must be an absolute URL without credentials, query or fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return "", errors.New("PUBLIC_URL must not contain a path")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return "", errors.New("PUBLIC_URL must use https unless it points to localhost")
		}
	default:
		return "", errors.New("PUBLIC_URL must use https")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

// sameResource compares resource indicators, accepting differently cased
// schemes and hosts and a trailing slash as recommended by the MCP spec.
func sameResource(a, b string) bool {
	normalize := func(raw string) (string, bool) {
		u, err := url.Parse(raw)
		if err != nil || !u.IsAbs() || u.Host == "" || u.User != nil || strings.Contains(raw, "#") {
			return "", false
		}
		path := strings.TrimSuffix(u.EscapedPath(), "/")
		return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + path + "?" + u.RawQuery, true
	}
	left, ok := normalize(a)
	if !ok {
		return false
	}
	right, ok := normalize(b)
	return ok && left == right
}
