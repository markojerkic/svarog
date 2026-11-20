package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type AuthorizationRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	State               string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resources           []string
}

// Authorization is a validated authorization request awaiting the user's decision.
type Authorization struct {
	Client        Client
	RedirectURI   string
	State         string
	Scope         string
	CodeChallenge string
	Resource      string
	issuer        string
}

// ClientError means the client or its redirect URI could not be verified. The
// user must see the error instead of being redirected (RFC 6749, 4.1.2.1).
type ClientError struct {
	Code        string
	Description string
}

func (e *ClientError) Error() string { return e.Code + ": " + e.Description }

// RedirectError is reported to the verified client through its redirect URI.
type RedirectError struct {
	Code        string
	Description string
	URL         string
}

func (e *RedirectError) Error() string { return e.Code + ": " + e.Description }

// ValidateAuthorization verifies an authorization request. It returns a
// *ClientError or *RedirectError for invalid requests.
func (s *Service) ValidateAuthorization(ctx context.Context, request AuthorizationRequest) (Authorization, error) {
	if request.ClientID == "" {
		return Authorization{}, &ClientError{Code: ErrorInvalidRequest, Description: "client_id is required"}
	}
	client, err := s.clients.Resolve(ctx, request.ClientID)
	switch {
	case errors.Is(err, ErrClientUnavailable):
		return Authorization{}, &ClientError{Code: ErrorTemporarilyUnavailable, Description: err.Error()}
	case errors.Is(err, ErrInvalidClient):
		return Authorization{}, &ClientError{Code: ErrorInvalidClient, Description: err.Error()}
	case err != nil:
		return Authorization{}, fmt.Errorf("resolve OAuth client: %w", err)
	}
	redirectURI := request.RedirectURI
	switch {
	case redirectURI == "" && len(client.RedirectURIs) == 1:
		// OAuth 2.1 lets a client with a single registered redirect URI omit it.
		redirectURI = client.RedirectURIs[0]
	case redirectURI == "":
		return Authorization{}, &ClientError{Code: ErrorInvalidRequest, Description: "redirect_uri is required"}
	case !redirectURIRegistered(client.RedirectURIs, redirectURI):
		return Authorization{}, &ClientError{Code: ErrorInvalidRequest, Description: "redirect_uri is not registered for this client"}
	}

	authorization := Authorization{Client: client, RedirectURI: redirectURI, State: request.State, issuer: s.issuer}
	redirectError := func(code, description string) error {
		return &RedirectError{Code: code, Description: description, URL: authorization.errorURL(code, description)}
	}
	if request.ResponseType != "code" {
		return Authorization{}, redirectError(ErrorUnsupportedResponseType, "response_type must be code")
	}
	if request.CodeChallengeMethod != "S256" || !validCodeChallenge(request.CodeChallenge) {
		return Authorization{}, redirectError(ErrorInvalidRequest, "PKCE with code_challenge_method S256 is required")
	}
	if !resourceRequested(request.Resources, s.resource) {
		return Authorization{}, redirectError(ErrorInvalidTarget, "resource must be "+s.resource)
	}
	// The only scope is granted regardless of the requested scopes (RFC 6749, 3.3).
	authorization.Scope = ScopeLogsRead
	authorization.CodeChallenge = request.CodeChallenge
	authorization.Resource = s.resource
	return authorization, nil
}

// Approve stores a single-use authorization code for the user and returns the
// client redirect URL that delivers it.
func (s *Service) Approve(ctx context.Context, userID uuid.UUID, authorization Authorization) (string, error) {
	code := newSecret(codePrefix)
	if err := s.queries.DeleteExpiredOAuthGrants(ctx); err != nil {
		return "", fmt.Errorf("delete expired OAuth grants: %w", err)
	}
	err := s.queries.CreateOAuthGrant(ctx, sqlc.CreateOAuthGrantParams{
		ID:            uuid.New(),
		UserID:        userID,
		ClientID:      authorization.Client.ID,
		RedirectUri:   authorization.RedirectURI,
		Resource:      authorization.Resource,
		Scope:         authorization.Scope,
		CodeHash:      hashSecret(code),
		CodeChallenge: authorization.CodeChallenge,
		CodeExpiresAt: time.Now().Add(codeTTL),
	})
	if err != nil {
		return "", fmt.Errorf("create OAuth grant: %w", err)
	}
	return authorization.responseURL(url.Values{"code": {code}}), nil
}

// DenyURL reports to the client that the user refused access.
func (a Authorization) DenyURL() string {
	return a.errorURL(ErrorAccessDenied, "The user denied the authorization request")
}

func (a Authorization) errorURL(code, description string) string {
	return a.responseURL(url.Values{"error": {code}, "error_description": {description}})
}

// responseURL adds state and the RFC 9207 issuer to the redirect URI's query.
func (a Authorization) responseURL(params url.Values) string {
	u, err := url.Parse(a.RedirectURI)
	if err != nil {
		// ValidateAuthorization accepted only parseable redirect URIs.
		panic(fmt.Sprintf("invalid validated redirect URI: %v", err))
	}
	query := u.Query()
	for key, values := range params {
		query[key] = values
	}
	if a.State != "" {
		query.Set("state", a.State)
	}
	query.Set("iss", a.issuer)
	u.RawQuery = query.Encode()
	return u.String()
}

// resourceRequested accepts an omitted resource or a single resource that
// identifies the expected one.
func resourceRequested(requested []string, expected string) bool {
	switch len(requested) {
	case 0:
		return true
	case 1:
		return sameResource(requested[0], expected)
	default:
		return false
	}
}
