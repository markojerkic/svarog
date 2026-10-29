package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/stretchr/testify/require"
)

type archiveCreateHTTPStub struct {
	archiveHTTPStub
	result        archive.CreateResult
	createErr     error
	listErr       error
	createdScope  archive.Scope
	listedScope   archive.Scope
	createContext context.Context
	query         archive.ListQuery
	calls         []string
}

func (s *archiveCreateHTTPStub) Create(ctx context.Context, scope archive.Scope) (archive.CreateResult, error) {
	s.calls = append(s.calls, "create")
	s.createdScope = scope
	s.createContext = ctx
	return s.result, s.createErr
}

func (s *archiveCreateHTTPStub) List(_ context.Context, scope archive.Scope, query archive.ListQuery) (archive.Page, error) {
	s.calls = append(s.calls, "list")
	s.listedScope, s.query = scope, query
	return s.page, s.listErr
}

func (s *archiveCreateHTTPStub) SetPolicy(context.Context, archive.Scope, archive.Policy) error {
	s.calls = append(s.calls, "setPolicy")
	return nil
}

func TestArchiveManualCreationRefreshesListWithoutSavingSettings(t *testing.T) {
	scope := archive.Scope{ProjectID: uuid.New(), ClientID: uuid.New()}
	completed := archive.Archive{ID: uuid.New(), ClientID: scope.ClientID, FileName: "manual.zip", CreatedAt: time.Now()}
	stub := &archiveCreateHTTPStub{
		archiveHTTPStub: archiveHTTPStub{page: archive.Page{Items: []archive.Archive{completed}}},
		result:          archive.CreateResult{Outcome: archive.Created, Archive: &completed},
	}
	server := newArchiveTestServer(t, stub)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodPost,
		"/logs/"+scope.ProjectID.String()+"/"+scope.ClientID.String()+"/archives/create",
		strings.NewReader("enabled=false&archiveAfterWeeks=999&clientId="+uuid.NewString()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("HX-Request", "true")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, "Manual creation must return the refreshed archive list after completion")
	require.Equal(t, []string{"create", "list"}, stub.calls, "Manual creation must build before listing and must not save submitted draft settings")
	require.Equal(t, scope, stub.createdScope, "Manual creation must use only the authorized project and client from the URL")
	require.Equal(t, scope, stub.listedScope, "The refreshed list must belong to the archived client")
	require.Same(t, ctx, stub.createContext, "The synchronous build must use the request context so cancellation reaches the archiver")
	require.Equal(t, archive.ListQuery{}, stub.query, "The refreshed list must restart at the newest completed archives")
	require.Contains(t, response.Body.String(), "/archives/"+completed.ID.String(), "The refreshed list must include the newly completed archive download")
	require.NotContains(t, response.Body.String(), "log-archive-settings-form", "Refreshing archives must preserve unsaved settings in the existing form")
	require.JSONEq(t, `{"toast":{"level":"success","message":"Archive created."}}`, response.Header().Get("HX-Trigger"), "Completion must notify the user only after the refreshed list is available")
}

func TestArchiveManualCreationSkipOutcomes(t *testing.T) {
	for _, test := range []struct {
		name    string
		outcome archive.CreateOutcome
		message string
	}{
		{name: "no eligible logs", outcome: archive.SkippedNoLogs, message: "No logs are older than the saved archive threshold."},
		{name: "disabled policy", outcome: archive.SkippedDisabled, message: "Enable and save archiving first."},
		{name: "another build owns the client", outcome: archive.SkippedBusy, message: "Archiving is already in progress for this client."},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &archiveCreateHTTPStub{result: archive.CreateResult{Outcome: test.outcome}}
			server := newArchiveTestServer(t, stub)
			request := httptest.NewRequest(http.MethodPost, "/logs/"+projectID+"/"+clientID+"/archives/create", nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			require.Equal(t, http.StatusNoContent, response.Code, "Expected skips must return 204 so HTMX preserves the existing archive list")
			require.Equal(t, []string{"create"}, stub.calls, "Skipped builds must not mutate settings or reload the user's current archive page")
			require.Empty(t, response.Body.String(), "Skipped builds must leave the current list and form untouched")
			require.JSONEq(t, `{"toast":{"level":"info","message":"`+test.message+`"}}`, response.Header().Get("HX-Trigger"), "Each skip outcome must explain why no archive was created")
		})
	}
}

func TestArchiveManualCreationFailures(t *testing.T) {
	for _, test := range []struct {
		name      string
		createErr error
		listErr   error
		status    int
		message   string
		calls     []string
	}{
		{name: "storage not configured", createErr: archive.ErrNotConfigured, status: http.StatusServiceUnavailable,
			message: "Archiving is not configured.", calls: []string{"create"}},
		{name: "invalid saved policy", createErr: archive.ErrInvalidInput, status: http.StatusBadRequest,
			message: "Enter a valid positive whole number of weeks.", calls: []string{"create"}},
		{name: "uncertain build failure", createErr: errors.New("private storage failure"), status: http.StatusInternalServerError,
			message: "Could not confirm the archiving result. Reopen the archive list before trying again.", calls: []string{"create"}},
		{name: "request canceled", createErr: context.Canceled, status: http.StatusInternalServerError,
			message: "Could not confirm the archiving result. Reopen the archive list before trying again.", calls: []string{"create"}},
		{name: "created but refresh failed", listErr: errors.New("private database failure"), status: http.StatusInternalServerError,
			message: "Archive created, but the list could not be refreshed. Reopen the archive panel.", calls: []string{"create", "list"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &archiveCreateHTTPStub{
				result: archive.CreateResult{Outcome: archive.Created}, createErr: test.createErr, listErr: test.listErr,
			}
			server := newArchiveTestServer(t, stub)
			request := httptest.NewRequest(http.MethodPost, "/logs/"+projectID+"/"+clientID+"/archives/create", nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			require.Equal(t, test.status, response.Code, "Manual creation failures must preserve the archive HTTP error contract")
			require.Equal(t, test.calls, stub.calls, "Failures must not cause another creation attempt or save draft settings")
			require.JSONEq(t, `{"toast":{"level":"error","message":"`+test.message+`"}}`, response.Header().Get("HX-Trigger"), "Failures must distinguish an uncertain build result from a completed build with a failed list refresh")
			require.NotContains(t, response.Body.String(), "private", "Internal storage and database errors must not leak to the user")
		})
	}
}

func TestArchiveManualCreationRequiresAuthorizedAdmin(t *testing.T) {
	admin := &auth.LoggedInUser{ID: uuid.NewString(), Role: auth.ADMIN}
	for _, test := range []struct {
		name     string
		user     *auth.LoggedInUser
		allowed  bool
		belongs  bool
		redirect string
	}{
		{name: "anonymous", redirect: "/login"},
		{name: "ordinary project member", user: &auth.LoggedInUser{ID: uuid.NewString(), Role: auth.USER}, allowed: true, belongs: true, redirect: "/"},
		{name: "no project access", user: admin, belongs: true, redirect: "/"},
		{name: "client from another project", user: admin, allowed: true, redirect: "/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &archiveCreateHTTPStub{}
			server := echo.New()
			handlers.NewArchiveRouter(stub,
				&projectAccessAuthStub{allowed: test.allowed},
				&projectAccessProjectsStub{belongs: test.belongs},
				loggedInGroup(server, test.user))
			request := httptest.NewRequest(http.MethodPost, "/logs/"+projectID+"/"+clientID+"/archives/create", nil)
			request.Header.Set("HX-Request", "true")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			require.Empty(t, stub.calls, "Unauthorized requests must never reach archive creation or list operations")
			require.Equal(t, http.StatusOK, response.Code, "Denied HTMX requests must return the existing redirect response status")
			require.Equal(t, test.redirect, response.Header().Get("HX-Redirect"), "Unauthorized manual creation must fully navigate away instead of swapping into the private URL")
			require.Empty(t, response.Header().Get("HX-Location"), "Authorization failures must not use HX-Location")
		})
	}
}

func TestArchiveManualCreationRejectsInvalidIDsAndGET(t *testing.T) {
	for _, test := range []struct {
		name    string
		method  string
		project string
		client  string
		status  int
	}{
		{name: "invalid project", method: http.MethodPost, project: "invalid", client: clientID, status: http.StatusBadRequest},
		{name: "noncanonical client", method: http.MethodPost, project: projectID, client: strings.ToUpper(clientID), status: http.StatusBadRequest},
		{name: "GET cannot create", method: http.MethodGet, project: projectID, client: clientID, status: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			stub := &archiveCreateHTTPStub{}
			server := newArchiveTestServer(t, stub)
			request := httptest.NewRequest(test.method, "/logs/"+test.project+"/"+test.client+"/archives/create", nil)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			require.Equal(t, test.status, response.Code, "Only a POST with canonical project and client IDs may reach manual creation")
			require.Empty(t, stub.calls, "Invalid scope and GET requests must not invoke the archiver")
		})
	}
}
