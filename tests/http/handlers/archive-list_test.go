package handlers_test

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/archivelink"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/stretchr/testify/require"
)

type archiveAccessAuth struct{}

func (archiveAccessAuth) UserHasAccessToProject(context.Context, string, string) (bool, error) {
	return true, nil
}

type archiveAccessProjects struct{}

func (archiveAccessProjects) ClientBelongsToProject(context.Context, string, string) (bool, error) {
	return true, nil
}

type archiveHTTPStub struct {
	archive.Archiver
	page         archive.Page
	download     archive.Download
	openErr      error
	opened       uuid.UUID
	closed       bool
	searches     []archive.SearchQuery
	searchResult archive.SearchResult
	searchErr    error
}

func (s *archiveHTTPStub) Search(_ context.Context, _ archive.Scope, query archive.SearchQuery) (archive.SearchResult, error) {
	s.searches = append(s.searches, query)
	return s.searchResult, s.searchErr
}

func TestArchiveSearchOlderPage(t *testing.T) {
	stub := &archiveHTTPStub{searchResult: archive.SearchResult{
		ArchivesScanned: 1, TotalMatches: 201, Hits: []archive.SearchHit{{Message: "first record"}}, OlderCursor: "next",
	}}
	server := newArchiveTestServer(t, stub)
	path := "/logs/" + uuid.NewString() + "/" + uuid.NewString() + "/archives/search"
	params := url.Values{"from": {"1700000000"}, "to": {"1710000000"}, "text": {"error & timeout"}}
	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path+"?"+params.Encode(), nil)
		if params.Has("older_cursor") {
			req.Header.Set("HX-Request", "true")
		}
		r := httptest.NewRecorder()
		server.ServeHTTP(r, req)
		return r
	}
	first := get()
	require.Equal(t, http.StatusOK, first.Code)
	require.Contains(t, first.Body.String(), `id="log-scroll-container"`)
	require.Contains(t, first.Body.String(), `id="log-search-dialog"`)
	params.Set("older_cursor", "next")
	require.Contains(t, html.UnescapeString(first.Body.String()), `hx-get="`+path+"?"+params.Encode()+`"`)
	require.Contains(t, first.Body.String(), "Load older")
	stub.searchResult = archive.SearchResult{
		ArchivesScanned: 1, TotalMatches: 201, Skipped: 200, Hits: []archive.SearchHit{{Message: "last record"}},
	}
	last := get()
	require.Equal(t, http.StatusOK, last.Code)
	require.Len(t, stub.searches, 2)
	require.Equal(t, "next", stub.searches[1].OlderCursor)
	require.Contains(t, last.Body.String(), "Showing 201 of 201")
	require.Contains(t, last.Body.String(), "last record")
	require.Contains(t, last.Body.String(), "data-log-message", "continuations use the shared log row")
	require.Contains(t, last.Body.String(), "data-archive-url", "archive actions must work on newly loaded rows")
	require.NotContains(t, last.Body.String(), "data-log-menu-trigger", "archive records are not live log IDs")
	require.NotContains(t, last.Body.String(), "first record")
	require.NotContains(t, last.Body.String(), `id="archive-search-form"`)
	require.NotContains(t, last.Body.String(), `id="archive-search-older"`)
	stub.searchErr = archive.ErrArchiveUnavailable
	retry := get()
	require.Equal(t, http.StatusServiceUnavailable, retry.Code)
	require.Contains(t, retry.Header().Get("HX-Trigger"), "could not be read or verified")
	stub.searchErr = archive.ErrSearchCursor
	stale := get()
	require.Equal(t, http.StatusConflict, stale.Code)
	require.Contains(t, stale.Body.String(), "Restart search")
	params.Del("older_cursor")
	require.Contains(t, html.UnescapeString(stale.Body.String()), `href="`+path+"?"+params.Encode()+`"`)
	require.NotContains(t, stale.Body.String(), "first record")
	require.NotContains(t, stale.Body.String(), "older_cursor")
}

func (s *archiveHTTPStub) GetPolicy(context.Context, archive.Scope) (archive.Policy, error) {
	return archive.Policy{Enabled: true, ArchiveAfterWeeks: 4}, nil
}

func (s *archiveHTTPStub) List(context.Context, archive.Scope, archive.ListQuery) (archive.Page, error) {
	return s.page, nil
}

func (s *archiveHTTPStub) Open(_ context.Context, _ archive.Scope, id uuid.UUID) (archive.Download, error) {
	s.opened = id
	if s.openErr != nil {
		return archive.Download{}, s.openErr
	}
	if id != s.download.Archive.ID {
		return archive.Download{}, archive.ErrArchiveNotFound
	}
	return s.download, nil
}

func newArchiveTestServer(t *testing.T, archiver archive.Archiver) *echo.Echo {
	t.Helper()
	e := echo.New()
	root := e.Group("")
	root.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(auth.UserKey.String(), &auth.LoggedInUser{ID: uuid.NewString(), Role: auth.ADMIN})
			return next(c)
		}
	})
	handlers.NewArchiveRouter(archiver, true, archiveAccessAuth{}, archiveAccessProjects{}, root)
	return e
}

func TestArchiveSearchFromURL(t *testing.T) {
	stub := &archiveHTTPStub{}
	server := newArchiveTestServer(t, stub)
	path := "/logs/" + uuid.NewString() + "/" + uuid.NewString() + "/archives/search"
	get := func(query url.Values) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path+"?"+query.Encode(), nil))
		return response
	}

	require.Equal(t, http.StatusOK, get(nil).Code)
	params := url.Values{"text": {"error & timeout"}}
	response := get(params)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "Select a date range to search the archive.")
	require.Empty(t, stub.searches)

	params.Set("from", "1774738800")
	params.Set("to", "1774821600")
	for _, requestType := range []string{"partial", "full"} {
		req := httptest.NewRequest(http.MethodGet, path+"?"+params.Encode(), nil)
		req.Header.Set("HX-Request", "true")
		req.Header.Set("HX-Request-Type", requestType)
		response = httptest.NewRecorder()
		server.ServeHTTP(response, req)
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Body.String(), `id="archive-results"`)
		if requestType == "partial" {
			require.NotContains(t, response.Body.String(), `id="archive-search-form"`)
		} else {
			require.Contains(t, response.Body.String(), `value="error &amp; timeout"`)
		}
	}
	require.Len(t, stub.searches, 2)
	require.Equal(t, stub.searches[0], stub.searches[1])
	require.Equal(t, "2026-03-28T23:00:00Z", stub.searches[0].From.UTC().Format(time.RFC3339))
	require.Equal(t, "2026-03-29T22:00:00Z", stub.searches[0].To.UTC().Format(time.RFC3339))
	require.Equal(t, params.Get("text"), stub.searches[0].Text)

	params.Set("to", params.Get("from"))
	response = get(params)
	require.Equal(t, http.StatusBadRequest, response.Code)
	params.Set("to", "not-a-timestamp")
	require.Equal(t, http.StatusBadRequest, get(params).Code)
	require.Len(t, stub.searches, 2)
}

func TestArchiveDownloadStreamsOnlyTheRequestedArchive(t *testing.T) {
	projectID := uuid.New()
	clientID := uuid.New()
	archiveID := uuid.New()
	content := &trackingReader{Reader: strings.NewReader("zip-bytes")}
	stub := &archiveHTTPStub{download: archive.Download{
		Archive: archive.Archive{ID: archiveID, FileName: `../secret/archive.zip`, SizeBytes: 9},
		Content: content,
	}}
	server := newArchiveTestServer(t, stub)

	request := httptest.NewRequest(http.MethodGet, "/logs/"+projectID.String()+"/"+clientID.String()+"/archives/"+archiveID.String(), nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, "A completed archive download must succeed")
	require.Equal(t, "zip-bytes", response.Body.String(), "The download must stream the archive content")
	require.Equal(t, "application/zip", response.Header().Get("Content-Type"), "The download must be a ZIP")
	require.Contains(t, response.Header().Get("Content-Disposition"), "archive.zip", "The download name must be the archive file name, not its storage path")
	require.NotContains(t, response.Header().Get("Content-Disposition"), "secret", "The download name must not include a directory path")
	require.Equal(t, archiveID, stub.opened, "The download must open the archive identified by the URL")
	require.True(t, content.closed, "The archive content must be closed after streaming")

	missing := httptest.NewRequest(http.MethodGet, "/logs/"+projectID.String()+"/"+clientID.String()+"/archives/"+uuid.NewString(), nil)
	missingResponse := httptest.NewRecorder()
	server.ServeHTTP(missingResponse, missing)
	require.Equal(t, http.StatusNotFound, missingResponse.Code, "A download for another archive must not return this archive")
	require.NotContains(t, missingResponse.Body.String(), "zip-bytes", "A missing archive must not leak another archive's content")
}

type archiveLinkAuth struct {
	archiveAccessAuth
	user auth.User
}

func (a *archiveLinkAuth) GetUserByID(context.Context, string) (auth.User, error) {
	return a.user, nil
}

func TestSignedArchiveDownload(t *testing.T) {
	users := &archiveLinkAuth{user: auth.User{ID: uuid.New(), Role: auth.ADMIN}}
	content := &trackingReader{Reader: strings.NewReader("zip-bytes")}
	archiveID := uuid.New()
	stub := &archiveHTTPStub{download: archive.Download{
		Archive: archive.Archive{ID: archiveID, FileName: "archive.zip", SizeBytes: 9},
		Content: content,
	}}
	signer := archivelink.NewSigner("archive-download-test-secret")
	claims := archivelink.Claims{
		UserID: users.user.ID, ProjectID: uuid.New(), ClientID: uuid.New(), ArchiveID: archiveID,
		ExpiresAt: time.Now().Add(archivelink.TTL),
	}
	token := signer.Sign(claims)
	server := echo.New()
	server.HTTPErrorHandler = middleware.CustomHTTPErrorHandler
	handlers.RegisterArchiveLinks(server, signer, users, archiveAccessProjects{}, stub)

	download := func(token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, archivelink.Path+token, nil)
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}

	response := download(token)
	require.Equal(t, http.StatusOK, response.Code, "A valid signed link must work without a session")
	require.Equal(t, "zip-bytes", response.Body.String(), "The signed link must stream the archive")

	stub.opened = uuid.Nil
	users.user.Role = auth.USER
	require.Equal(t, http.StatusNotFound, download(token).Code, "Losing the administrator role must invalidate a previously issued link")
	users.user.Role = auth.ADMIN
	claims.ExpiresAt = time.Now().Add(-time.Minute)
	require.Equal(t, http.StatusNotFound, download(signer.Sign(claims)).Code, "An expired link must be rejected")
	claims.ExpiresAt = time.Now().Add(archivelink.TTL)
	otherToken := archivelink.NewSigner("another-secret").Sign(claims)
	require.Equal(t, http.StatusNotFound, download(otherToken).Code, "A link with an invalid signature must be rejected")
	require.Equal(t, uuid.Nil, stub.opened, "Rejected links must never open an archive")

	var logs bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(originalLogger) })
	stub.openErr = errors.New("archive store failed")
	require.Equal(t, http.StatusInternalServerError, download(token).Code, "A storage failure must return a server error")
	require.Contains(t, logs.String(), "archive store failed", "Storage failures must be recorded for diagnosis")
	require.NotContains(t, logs.String(), token, "HTTP error logs must not expose the signed link")
}

type trackingReader struct {
	io.Reader
	closed bool
}

func (r *trackingReader) Close() error {
	r.closed = true
	return nil
}
