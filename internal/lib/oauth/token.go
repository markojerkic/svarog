package oauth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

type TokenRequest struct {
	GrantType    string
	ClientID     string
	Code         string
	RedirectURI  string
	CodeVerifier string
	RefreshToken string
	Resources    []string
	// ClientAuthenticated reports client credentials in the request. Clients
	// identified by metadata documents are public and must not send any.
	ClientAuthenticated bool
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// TokenError is an RFC 6749 token endpoint error.
type TokenError struct {
	Code        string
	Description string
}

func (e *TokenError) Error() string { return e.Code + ": " + e.Description }

// Token handles the authorization_code and refresh_token grants. Invalid
// requests return a *TokenError.
func (s *Service) Token(ctx context.Context, request TokenRequest) (TokenResponse, error) {
	if request.ClientAuthenticated {
		return TokenResponse{}, &TokenError{Code: ErrorInvalidClient, Description: "public clients must not send client credentials"}
	}
	if request.ClientID == "" {
		return TokenResponse{}, &TokenError{Code: ErrorInvalidRequest, Description: "client_id is required"}
	}
	switch request.GrantType {
	case "authorization_code":
		return s.exchangeCode(ctx, request)
	case "refresh_token":
		return s.refresh(ctx, request)
	case "":
		return TokenResponse{}, &TokenError{Code: ErrorInvalidRequest, Description: "grant_type is required"}
	default:
		return TokenResponse{}, &TokenError{Code: ErrorUnsupportedGrantType, Description: "grant_type must be authorization_code or refresh_token"}
	}
}

func (s *Service) exchangeCode(ctx context.Context, request TokenRequest) (TokenResponse, error) {
	// OAuth 2.1 drops redirect_uri from the token request; PKCE binds the code
	// to the client instead. A supplied redirect_uri must still match.
	if request.Code == "" || request.CodeVerifier == "" {
		return TokenResponse{}, &TokenError{Code: ErrorInvalidRequest, Description: "code and code_verifier are required"}
	}
	grant, err := s.queries.GetOAuthGrantByCode(ctx, hashSecret(request.Code))
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenResponse{}, invalidGrant("authorization code is invalid")
	}
	if err != nil {
		return TokenResponse{}, fmt.Errorf("get OAuth grant by code: %w", err)
	}
	if grant.Exchanged {
		// A reused code may have been stolen; revoke the tokens issued for it.
		return TokenResponse{}, s.revoke(ctx, grant.ID, "authorization code was already used")
	}
	if !time.Now().Before(grant.CodeExpiresAt) {
		return TokenResponse{}, invalidGrant("authorization code expired")
	}
	if grant.ClientID != request.ClientID || (request.RedirectURI != "" && grant.RedirectUri != request.RedirectURI) {
		return TokenResponse{}, invalidGrant("authorization code was issued to another client or redirect_uri")
	}
	if !resourceRequested(request.Resources, grant.Resource) {
		return TokenResponse{}, &TokenError{Code: ErrorInvalidTarget, Description: "resource does not match the authorization"}
	}
	if !verifyCodeVerifier(request.CodeVerifier, grant.CodeChallenge) {
		return TokenResponse{}, invalidGrant("code_verifier does not match the code_challenge")
	}

	tokens := newIssuedTokens(grant.ID, grant.Scope)
	updated, err := s.queries.IssueOAuthGrantTokens(ctx, sqlc.IssueOAuthGrantTokensParams{
		ID:               grant.ID,
		AccessTokenHash:  hashSecret(tokens.AccessToken),
		AccessExpiresAt:  tokens.accessExpiresAt,
		RefreshTokenHash: hashSecret(tokens.RefreshToken),
		ExpiresAt:        tokens.refreshExpiresAt,
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("issue OAuth tokens: %w", err)
	}
	if updated == 0 {
		return TokenResponse{}, s.revoke(ctx, grant.ID, "authorization code was already used")
	}
	return tokens.TokenResponse, nil
}

func (s *Service) refresh(ctx context.Context, request TokenRequest) (TokenResponse, error) {
	if request.RefreshToken == "" {
		return TokenResponse{}, &TokenError{Code: ErrorInvalidRequest, Description: "refresh_token is required"}
	}
	grantID, ok := refreshTokenGrant(request.RefreshToken)
	if !ok {
		return TokenResponse{}, invalidGrant("refresh token is invalid")
	}
	presented := hashSecret(request.RefreshToken)
	grant, err := s.queries.GetOAuthGrantForRefresh(ctx, sqlc.GetOAuthGrantForRefreshParams{ID: grantID, TokenHash: presented})
	if errors.Is(err, pgx.ErrNoRows) {
		return TokenResponse{}, invalidGrant("refresh token is invalid")
	}
	if err != nil {
		return TokenResponse{}, fmt.Errorf("get OAuth grant for refresh: %w", err)
	}
	if !grant.IsCurrent {
		// Only holders of one of the grant's tokens know its random ID, and
		// after a rotation only one party can hold the current token.
		return TokenResponse{}, s.revoke(ctx, grant.ID, "refresh token was already used")
	}
	if grant.ClientID != request.ClientID {
		return TokenResponse{}, invalidGrant("refresh token was issued to another client")
	}
	if !time.Now().Before(grant.ExpiresAt) {
		return TokenResponse{}, invalidGrant("refresh token expired")
	}
	if !resourceRequested(request.Resources, grant.Resource) {
		return TokenResponse{}, &TokenError{Code: ErrorInvalidTarget, Description: "resource does not match the authorization"}
	}

	tokens := newIssuedTokens(grant.ID, grant.Scope)
	updated, err := s.queries.RotateOAuthGrantTokens(ctx, sqlc.RotateOAuthGrantTokensParams{
		ID:                  grant.ID,
		RefreshTokenHash:    presented,
		NewRefreshTokenHash: hashSecret(tokens.RefreshToken),
		AccessTokenHash:     hashSecret(tokens.AccessToken),
		AccessExpiresAt:     tokens.accessExpiresAt,
		ExpiresAt:           tokens.refreshExpiresAt,
	})
	if err != nil {
		return TokenResponse{}, fmt.Errorf("rotate OAuth tokens: %w", err)
	}
	if updated == 0 {
		return TokenResponse{}, s.revoke(ctx, grant.ID, "refresh token was already used")
	}
	return tokens.TokenResponse, nil
}

func (s *Service) revoke(ctx context.Context, grantID uuid.UUID, reason string) error {
	if err := s.queries.DeleteOAuthGrant(ctx, grantID); err != nil {
		return fmt.Errorf("revoke OAuth grant: %w", err)
	}
	return invalidGrant(reason)
}

func invalidGrant(description string) error {
	return &TokenError{Code: ErrorInvalidGrant, Description: description}
}

type issuedTokens struct {
	TokenResponse
	accessExpiresAt  time.Time
	refreshExpiresAt time.Time
}

func newIssuedTokens(grantID uuid.UUID, scope string) issuedTokens {
	now := time.Now()
	return issuedTokens{
		TokenResponse: TokenResponse{
			AccessToken:  newSecret(accessTokenPrefix),
			TokenType:    "Bearer",
			ExpiresIn:    int64(accessTokenTTL / time.Second),
			RefreshToken: newRefreshToken(grantID),
			Scope:        scope,
		},
		accessExpiresAt:  now.Add(accessTokenTTL),
		refreshExpiresAt: now.Add(refreshTokenTTL),
	}
}

// ErrInvalidAccessToken means the token is unknown, expired, revoked or not
// issued for this resource.
var ErrInvalidAccessToken = errors.New("invalid access token")

type AccessToken struct {
	UserID    uuid.UUID
	Scopes    []string
	ExpiresAt time.Time
}

// VerifyAccessToken checks the token against its stored grant on every call,
// so revoking the grant takes effect immediately.
func (s *Service) VerifyAccessToken(ctx context.Context, token string) (AccessToken, error) {
	if !strings.HasPrefix(token, accessTokenPrefix) {
		return AccessToken{}, ErrInvalidAccessToken
	}
	grant, err := s.queries.GetOAuthAccessGrant(ctx, hashSecret(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return AccessToken{}, ErrInvalidAccessToken
	}
	if err != nil {
		return AccessToken{}, fmt.Errorf("get OAuth access grant: %w", err)
	}
	// A user who must reset their password cannot use the web UI either.
	if grant.NeedsPasswordReset || !sameResource(grant.Resource, s.resource) {
		return AccessToken{}, ErrInvalidAccessToken
	}
	return AccessToken{UserID: grant.UserID, Scopes: strings.Fields(grant.Scope), ExpiresAt: grant.AccessExpiresAt}, nil
}
