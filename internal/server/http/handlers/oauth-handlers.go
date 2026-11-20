package handlers

import (
	"context"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	authpages "github.com/markojerkic/svarog/internal/server/ui/pages/auth"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type oauthService interface {
	Metadata() oauth.ServerMetadata
	ValidateAuthorization(context.Context, oauth.AuthorizationRequest) (oauth.Authorization, error)
	Approve(context.Context, uuid.UUID, oauth.Authorization) (string, error)
	Token(context.Context, oauth.TokenRequest) (oauth.TokenResponse, error)
}

type oauthRouter struct {
	service oauthService
}

const maxTokenRequestBytes = 64 << 10

// Parameters of the authorization request carried through the consent form.
var authorizationParams = []string{
	"response_type", "client_id", "redirect_uri", "state", "scope",
	"code_challenge", "code_challenge_method", "resource",
}

// RegisterOAuth serves the authorization server for MCP clients. Metadata and
// the token endpoint are public; authorization requires a signed-in user.
func RegisterOAuth(service oauthService, e *echo.Echo, private *echo.Group) {
	router := &oauthRouter{service: service}
	e.GET(oauth.MetadataPath, router.metadata)
	e.OPTIONS(oauth.MetadataPath, router.preflight)
	e.POST(oauth.TokenPath, router.token)
	e.OPTIONS(oauth.TokenPath, router.preflight)
	private.GET(oauth.AuthorizationPath, router.authorizePage)
	private.POST(oauth.AuthorizationPath, router.authorize)
}

func (r *oauthRouter) metadata(c *echo.Context) error {
	allowAnyOrigin(c)
	return c.JSON(http.StatusOK, r.service.Metadata())
}

func (r *oauthRouter) preflight(c *echo.Context) error {
	allowAnyOrigin(c)
	return c.NoContent(http.StatusNoContent)
}

func (r *oauthRouter) authorizePage(c *echo.Context) error {
	params := c.QueryParams()
	authorization, err := r.validateAuthorization(c.Request().Context(), params)
	if err != nil {
		return authorizationFailure(c, err)
	}
	user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
	if !ok || user == nil {
		return htmx.Redirect(c, "/login")
	}

	header := c.Response().Header()
	header.Set("Content-Security-Policy", "frame-ancestors 'none'")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Referrer-Policy", "no-referrer")
	redirect, _ := url.Parse(authorization.RedirectURI)
	clientID, _ := url.Parse(authorization.Client.ID)
	props := authpages.OAuthConsentPageProps{
		ClientName:    authorization.Client.Name,
		ClientID:      authorization.Client.ID,
		ClientHost:    clientID.Host,
		RedirectHost:  redirect.Host,
		LocalRedirect: redirect.Scheme == "http",
		Username:      user.Username,
		Action:        oauth.AuthorizationPath,
	}
	for _, name := range authorizationParams {
		for _, value := range params[name] {
			props.Fields = append(props.Fields, authpages.OAuthField{Name: name, Value: value})
		}
	}
	return utils.Render(c, http.StatusOK, authpages.OAuthConsentPage(props))
}

func (r *oauthRouter) authorize(c *echo.Context) error {
	if err := http.NewCrossOriginProtection().Check(c.Request()); err != nil {
		return c.NoContent(http.StatusForbidden)
	}
	if err := c.Request().ParseForm(); err != nil {
		return utils.Render(c, http.StatusBadRequest, pages.ErrorPage(pages.ErrorPageProps{StatusCode: http.StatusBadRequest}))
	}
	form := c.Request().PostForm
	ctx := c.Request().Context()
	authorization, err := r.validateAuthorization(ctx, form)
	if err != nil {
		return authorizationFailure(c, err)
	}
	if form.Get("decision") != "allow" {
		return redirectToClient(c, authorization.DenyURL())
	}
	user, ok := c.Get(auth.UserKey.String()).(*auth.LoggedInUser)
	if !ok || user == nil {
		return htmx.Redirect(c, "/login")
	}
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		return err
	}
	target, err := r.service.Approve(ctx, userID, authorization)
	if err != nil {
		return err
	}
	return redirectToClient(c, target)
}

func (r *oauthRouter) token(c *echo.Context) error {
	allowAnyOrigin(c)
	header := c.Response().Header()
	header.Set("Cache-Control", "no-store")
	header.Set("Pragma", "no-cache")

	request := c.Request()
	mediaType, _, _ := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if mediaType != "application/x-www-form-urlencoded" {
		return tokenError(c, &oauth.TokenError{Code: oauth.ErrorInvalidRequest, Description: "the request body must be application/x-www-form-urlencoded"})
	}
	request.Body = http.MaxBytesReader(c.Response(), request.Body, maxTokenRequestBytes)
	if err := request.ParseForm(); err != nil {
		return tokenError(c, &oauth.TokenError{Code: oauth.ErrorInvalidRequest, Description: "the request body is invalid"})
	}
	form := request.PostForm
	for name, values := range form {
		if len(values) > 1 && name != "resource" {
			return tokenError(c, &oauth.TokenError{Code: oauth.ErrorInvalidRequest, Description: name + " must not be repeated"})
		}
	}
	response, err := r.service.Token(request.Context(), oauth.TokenRequest{
		GrantType:    form.Get("grant_type"),
		ClientID:     form.Get("client_id"),
		Code:         form.Get("code"),
		RedirectURI:  form.Get("redirect_uri"),
		CodeVerifier: form.Get("code_verifier"),
		RefreshToken: form.Get("refresh_token"),
		Resources:    form["resource"],
		ClientAuthenticated: request.Header.Get("Authorization") != "" ||
			form.Has("client_secret") || form.Has("client_assertion"),
	})
	var oauthErr *oauth.TokenError
	if errors.As(err, &oauthErr) {
		return tokenError(c, oauthErr)
	}
	if err != nil {
		slog.Error("OAuth token request failed", "error", err)
		return c.JSON(http.StatusInternalServerError, tokenErrorBody{Error: "server_error"})
	}
	return c.JSON(http.StatusOK, response)
}

type tokenErrorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func tokenError(c *echo.Context, err *oauth.TokenError) error {
	status := http.StatusBadRequest
	if err.Code == oauth.ErrorInvalidClient {
		if scheme := authorizationScheme(c.Request().Header.Get("Authorization")); scheme != "" {
			// RFC 6749, 5.2: rejected header authentication gets a 401
			// challenge for the scheme the client used.
			c.Response().Header().Set("WWW-Authenticate", scheme+` realm="Svarog"`)
			status = http.StatusUnauthorized
		}
	}
	return c.JSON(status, tokenErrorBody{Error: err.Code, ErrorDescription: err.Description})
}

// authorizationScheme returns the scheme of an Authorization header, or Basic
// when the scheme is not a valid HTTP token.
func authorizationScheme(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	scheme, _, _ := strings.Cut(header, " ")
	for _, r := range scheme {
		if r > unicode.MaxASCII || !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return "Basic"
		}
	}
	return scheme
}

// validateAuthorization rejects repeated parameters before validating the
// request (RFC 6749, 3.1). With an ambiguous client or redirect URI, the
// error must not be sent to either one.
func (r *oauthRouter) validateAuthorization(ctx context.Context, params url.Values) (oauth.Authorization, error) {
	for _, name := range authorizationParams {
		if name != "resource" && len(params[name]) > 1 {
			return oauth.Authorization{}, &oauth.ClientError{Code: oauth.ErrorInvalidRequest, Description: name + " must not be repeated"}
		}
	}
	return r.service.ValidateAuthorization(ctx, authorizationRequest(params))
}

func authorizationRequest(params url.Values) oauth.AuthorizationRequest {
	return oauth.AuthorizationRequest{
		ResponseType:        params.Get("response_type"),
		ClientID:            params.Get("client_id"),
		RedirectURI:         params.Get("redirect_uri"),
		State:               params.Get("state"),
		Scope:               params.Get("scope"),
		CodeChallenge:       params.Get("code_challenge"),
		CodeChallengeMethod: params.Get("code_challenge_method"),
		Resources:           slices.Clone(params["resource"]),
	}
}

// authorizationFailure redirects errors to a verified client. When the client
// or its redirect URI cannot be verified, the user sees the error instead.
func authorizationFailure(c *echo.Context, err error) error {
	var redirectErr *oauth.RedirectError
	if errors.As(err, &redirectErr) {
		return redirectToClient(c, redirectErr.URL)
	}
	var clientErr *oauth.ClientError
	if !errors.As(err, &clientErr) {
		return err
	}
	slog.Warn("OAuth authorization request rejected", "error", clientErr.Code, "reason", clientErr.Description)
	status, key := http.StatusBadRequest, "oauth.invalid_request"
	switch clientErr.Code {
	case oauth.ErrorInvalidClient:
		key = "oauth.invalid_client"
	case oauth.ErrorTemporarilyUnavailable:
		status, key = http.StatusServiceUnavailable, "oauth.client_unavailable"
	}
	return utils.Render(c, status, pages.ErrorPage(pages.ErrorPageProps{
		StatusCode: status,
		Message:    i18n.T(c.Request().Context(), key),
	}))
}

// redirectToClient leaves Svarog for the client's redirect URI. A 303 keeps a
// POSTed consent form from being resubmitted to the client.
func redirectToClient(c *echo.Context, target string) error {
	if htmx.IsHXRequest(c) {
		c.Response().Header().Set("HX-Redirect", target)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

// OAuth metadata and token responses contain no cookies, so browser-based MCP
// clients may read them from any origin.
func allowAnyOrigin(c *echo.Context) {
	header := c.Response().Header()
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	header.Set("Access-Control-Allow-Headers", "Content-Type")
}
