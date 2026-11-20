package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	// ErrInvalidClient means the client's metadata is missing, invalid or not allowed.
	ErrInvalidClient = errors.New("invalid OAuth client")
	// ErrClientUnavailable means the client's metadata could not be fetched right now.
	ErrClientUnavailable = errors.New("OAuth client metadata unavailable")
)

// Client is a verified OAuth client.
type Client struct {
	ID           string // Client ID Metadata Document URL
	Name         string
	RedirectURIs []string
}

type ClientResolver interface {
	Resolve(ctx context.Context, clientID string) (Client, error)
}

const (
	maxClientIDLength       = 2048
	maxClientNameLength     = 100
	maxRedirectURIs         = 20
	maxMetadataDocumentSize = 5 << 10
	metadataFetchTimeout    = 5 * time.Second
	minMetadataCacheTTL     = 5 * time.Minute
	defaultMetadataCacheTTL = time.Hour
	maxMetadataCacheTTL     = 24 * time.Hour
	maxCachedClients        = 1024
)

// MetadataDocumentResolver fetches and caches Client ID Metadata Documents.
// It never connects to special-use addresses, so a client_id URL cannot make
// Svarog request internal services. Each replica has its own cache.
type MetadataDocumentResolver struct {
	http  *http.Client
	mu    sync.Mutex
	cache map[string]cachedClient
}

type cachedClient struct {
	client    Client
	expiresAt time.Time
}

func NewMetadataDocumentResolver() *MetadataDocumentResolver {
	dialer := &net.Dialer{Timeout: metadataFetchTimeout, Control: rejectSpecialUseAddress}
	transport := &http.Transport{
		// A proxy would connect on our behalf and bypass the address check.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   metadataFetchTimeout,
		ResponseHeaderTimeout: metadataFetchTimeout,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
	}
	return &MetadataDocumentResolver{
		http: &http.Client{
			Transport: transport,
			Timeout:   metadataFetchTimeout,
			// The document must be served directly from its URL.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		cache: map[string]cachedClient{},
	}
}

func (r *MetadataDocumentResolver) Resolve(ctx context.Context, clientID string) (Client, error) {
	if err := ValidateClientID(clientID); err != nil {
		return Client{}, err
	}
	if client, ok := r.cached(clientID); ok {
		return client, nil
	}
	client, ttl, err := r.fetch(ctx, clientID)
	if err != nil {
		return Client{}, err
	}
	r.store(clientID, client, ttl)
	return client, nil
}

func (r *MetadataDocumentResolver) fetch(ctx context.Context, clientID string) (Client, time.Duration, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return Client{}, 0, fmt.Errorf("%w: %v", ErrInvalidClient, err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := r.http.Do(request)
	if errors.Is(err, errSpecialUseAddress) {
		return Client{}, 0, fmt.Errorf("%w: client_id resolves to a special-use address", ErrInvalidClient)
	}
	if err != nil {
		return Client{}, 0, fmt.Errorf("%w: %v", ErrClientUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		return Client{}, 0, fmt.Errorf("%w: metadata document returned HTTP %d", ErrClientUnavailable, response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return Client{}, 0, fmt.Errorf("%w: metadata document returned HTTP %d", ErrInvalidClient, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxMetadataDocumentSize+1))
	if err != nil {
		return Client{}, 0, fmt.Errorf("%w: read metadata document: %v", ErrClientUnavailable, err)
	}
	if len(body) > maxMetadataDocumentSize {
		return Client{}, 0, fmt.Errorf("%w: metadata document exceeds %d bytes", ErrInvalidClient, maxMetadataDocumentSize)
	}
	client, err := ParseMetadataDocument(clientID, body)
	if err != nil {
		return Client{}, 0, err
	}
	return client, metadataCacheTTL(response.Header.Values("Cache-Control")), nil
}

func (r *MetadataDocumentResolver) cached(clientID string) (Client, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.cache[clientID]
	if !ok || !time.Now().Before(entry.expiresAt) {
		return Client{}, false
	}
	return entry.client, true
}

func (r *MetadataDocumentResolver) store(clientID string, client Client, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if len(r.cache) >= maxCachedClients {
		for key, entry := range r.cache {
			if !now.Before(entry.expiresAt) {
				delete(r.cache, key)
			}
		}
	}
	if len(r.cache) >= maxCachedClients {
		for key := range r.cache {
			delete(r.cache, key)
			break
		}
	}
	r.cache[clientID] = cachedClient{client: client, expiresAt: now.Add(ttl)}
}

// ValidateClientID checks the Client Identifier URL rules of the Client ID
// Metadata Document specification. The URL is compared as an exact string.
func ValidateClientID(clientID string) error {
	invalid := func(reason string) error { return fmt.Errorf("%w: client_id %s", ErrInvalidClient, reason) }
	if clientID == "" || len(clientID) > maxClientIDLength {
		return invalid("must be a URL of at most 2048 bytes")
	}
	if !strings.HasPrefix(clientID, "https://") {
		return invalid("must use https")
	}
	u, err := url.Parse(clientID)
	if err != nil || u.Host == "" || u.Hostname() == "" || u.Opaque != "" {
		return invalid("must be an absolute URL")
	}
	if u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(clientID, "#") {
		return invalid("must not contain credentials, a query or a fragment")
	}
	if u.Path == "" || u.Path == "/" {
		return invalid("must have a path")
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return invalid("must not contain dot segments")
		}
	}
	return nil
}

type metadataDocument struct {
	ClientID                          string          `json:"client_id"`
	ClientName                        string          `json:"client_name"`
	RedirectURIs                      []string        `json:"redirect_uris"`
	GrantTypes                        []string        `json:"grant_types"`
	ResponseTypes                     []string        `json:"response_types"`
	TokenEndpointAuthMethod           *string         `json:"token_endpoint_auth_method"`
	TokenEndpointAuthMethodsSupported []string        `json:"token_endpoint_auth_methods_supported"`
	ClientSecret                      json.RawMessage `json:"client_secret"`
	ClientSecretExpiresAt             json.RawMessage `json:"client_secret_expires_at"`
}

// ParseMetadataDocument validates a fetched Client ID Metadata Document. Svarog
// supports only public clients using the authorization code grant.
func ParseMetadataDocument(clientID string, body []byte) (Client, error) {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", ErrInvalidClient, reason) }
	var document metadataDocument
	if err := json.Unmarshal(body, &document); err != nil {
		return Client{}, invalid("metadata document is not a JSON object")
	}
	if document.ClientID != clientID {
		return Client{}, invalid("client_id in the metadata document does not match its URL")
	}
	name := strings.TrimSpace(document.ClientName)
	if name == "" || utf8.RuneCountInString(name) > maxClientNameLength || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Client{}, invalid("client_name is missing or invalid")
	}
	if len(document.RedirectURIs) == 0 || len(document.RedirectURIs) > maxRedirectURIs {
		return Client{}, invalid("redirect_uris must list between 1 and 20 URIs")
	}
	for _, redirectURI := range document.RedirectURIs {
		if !validRedirectURI(redirectURI) {
			return Client{}, invalid("redirect_uris must use https or a localhost http URL")
		}
	}
	if document.ClientSecret != nil || document.ClientSecretExpiresAt != nil {
		return Client{}, invalid("metadata document must not contain a client secret")
	}
	switch {
	case document.TokenEndpointAuthMethodsSupported != nil:
		if !slices.Contains(document.TokenEndpointAuthMethodsSupported, "none") {
			return Client{}, invalid("client must support token endpoint authentication method none")
		}
	case document.TokenEndpointAuthMethod != nil && *document.TokenEndpointAuthMethod != "none":
		return Client{}, invalid("client must use token endpoint authentication method none")
	}
	if document.GrantTypes != nil && !slices.Contains(document.GrantTypes, "authorization_code") {
		return Client{}, invalid("grant_types must include authorization_code")
	}
	if document.ResponseTypes != nil && !slices.Contains(document.ResponseTypes, "code") {
		return Client{}, invalid("response_types must include code")
	}
	return Client{ID: clientID, Name: name, RedirectURIs: document.RedirectURIs}, nil
}

// metadataCacheTTL honors max-age within fixed bounds, so a client cannot force
// a fetch on every request or keep stale metadata indefinitely.
func metadataCacheTTL(cacheControl []string) time.Duration {
	ttl := defaultMetadataCacheTTL
	for _, header := range cacheControl {
		for _, directive := range strings.Split(header, ",") {
			directive = strings.ToLower(strings.TrimSpace(directive))
			switch {
			case directive == "no-store" || directive == "no-cache":
				return minMetadataCacheTTL
			case strings.HasPrefix(directive, "max-age="):
				seconds, err := strconv.Atoi(strings.Trim(strings.TrimPrefix(directive, "max-age="), `"`))
				if err == nil && seconds >= 0 {
					ttl = time.Duration(seconds) * time.Second
				}
			}
		}
	}
	return min(max(ttl, minMetadataCacheTTL), maxMetadataCacheTTL)
}
