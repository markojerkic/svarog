package handlers_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/stretchr/testify/require"
)

type archiveAccessAuth struct{ auth.AuthService }

func (archiveAccessAuth) UserHasAccessToProject(context.Context, string, string) (bool, error) {
	return true, nil
}

type archiveAccessProjects struct{ projects.ProjectsService }

func (archiveAccessProjects) ClientBelongsToProject(context.Context, string, string) (bool, error) {
	return true, nil
}

type archiveHTTPStub struct {
	archive.Archiver
	page     archive.Page
	download archive.Download
	openErr  error
	opened   uuid.UUID
	closed   bool
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
	handlers.NewArchiveRouter(archiver, archiveAccessAuth{}, archiveAccessProjects{}, root)
	return e
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

type trackingReader struct {
	io.Reader
	closed bool
}

func (r *trackingReader) Close() error {
	r.closed = true
	return nil
}
