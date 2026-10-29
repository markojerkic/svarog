package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/stretchr/testify/require"
)

const (
	projectID = "a382bf10-a951-4c30-909e-33ff36191d2c"
	clientID  = "c1e462e7-542a-497e-9632-f406ca03e786"
)

type projectAccessAuthStub struct {
	auth.AuthService
	allowed bool
	err     error
	calls   int
	userID  string
	project string
}

func (s *projectAccessAuthStub) UserHasAccessToProject(_ context.Context, userID, project string) (bool, error) {
	s.calls++
	s.userID, s.project = userID, project
	return s.allowed, s.err
}

type projectAccessProjectsStub struct {
	projects.ProjectsService
	belongs       bool
	err           error
	calls         int
	project       string
	client        string
	visible       []projects.Project
	allCalls      int
	assignedCalls int
	assignedIDs   []string
	deleteErr     error
}

func (s *projectAccessProjectsStub) ClientBelongsToProject(_ context.Context, project, client string) (bool, error) {
	s.calls++
	s.project, s.client = project, client
	return s.belongs, s.err
}

func (s *projectAccessProjectsStub) GetProjects(context.Context) ([]projects.Project, error) {
	s.allCalls++
	return s.visible, nil
}

func (s *projectAccessProjectsStub) GetProjectsByIds(_ context.Context, ids []string) ([]projects.Project, error) {
	s.assignedCalls++
	s.assignedIDs = ids
	return s.visible, nil
}

func (s *projectAccessProjectsStub) DeleteProject(context.Context, string) error {
	return s.deleteErr
}

type projectAccessLogsStub struct {
	db.LogService
	calls int
}

func (s *projectAccessLogsStub) GetInstances(context.Context, string, string) ([]string, error) {
	s.calls++
	return []string{"instance"}, nil
}

func loggedInGroup(e *echo.Echo, user *auth.LoggedInUser) *echo.Group {
	return e.Group("", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(auth.UserKey.String(), user)
			ctx := context.WithValue(c.Request().Context(), auth.UserKey, user)
			ctx = context.WithValue(ctx, auth.IsAdminKey, user != nil && user.Role == auth.ADMIN)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
}

func TestProjectAccessMiddleware(t *testing.T) {
	tests := []struct {
		name         string
		user         *auth.LoggedInUser
		project      string
		client       string
		allowed      bool
		belongs      bool
		authErr      error
		projectErr   error
		status       int
		redirect     string
		authCalls    int
		projectCalls int
	}{
		{name: "assigned user allowed without cached assignments", user: &auth.LoggedInUser{ID: "user", Role: auth.USER}, allowed: true, belongs: true, status: http.StatusNoContent, authCalls: 1, projectCalls: 1},
		{name: "unassigned user denied despite cached assignment", user: &auth.LoggedInUser{ID: "user", Role: auth.USER, ProjectIDs: []string{projectID}}, status: http.StatusOK, redirect: "/", authCalls: 1},
		{name: "admin allowed for existing client including historical inactive client", user: &auth.LoggedInUser{ID: "admin", Role: auth.ADMIN}, allowed: true, belongs: true, status: http.StatusNoContent, authCalls: 1, projectCalls: 1},
		{name: "admin denied for missing project", user: &auth.LoggedInUser{ID: "admin", Role: auth.ADMIN}, project: uuid.NewString(), allowed: true, status: http.StatusOK, redirect: "/", authCalls: 1, projectCalls: 1},
		{name: "admin denied for missing client", user: &auth.LoggedInUser{ID: "admin", Role: auth.ADMIN}, client: uuid.NewString(), allowed: true, status: http.StatusOK, redirect: "/", authCalls: 1, projectCalls: 1},
		{name: "cross-project client denied", user: &auth.LoggedInUser{ID: "user", Role: auth.USER}, allowed: true, status: http.StatusOK, redirect: "/", authCalls: 1, projectCalls: 1},
		{name: "anonymous redirected to login", status: http.StatusOK, redirect: "/login"},
		{name: "wildcard project rejected before queries", user: &auth.LoggedInUser{Role: auth.ADMIN}, project: "*", status: http.StatusBadRequest},
		{name: "wildcard client rejected before queries", user: &auth.LoggedInUser{Role: auth.ADMIN}, client: ">", status: http.StatusBadRequest},
		{name: "noncanonical UUID rejected before queries", user: &auth.LoggedInUser{Role: auth.ADMIN}, project: strings.ReplaceAll(projectID, "-", ""), status: http.StatusBadRequest},
		{name: "uppercase UUID rejected before queries", user: &auth.LoggedInUser{Role: auth.ADMIN}, client: strings.ToUpper(clientID), status: http.StatusBadRequest},
		{name: "nil UUID rejected before queries", user: &auth.LoggedInUser{Role: auth.ADMIN}, client: uuid.Nil.String(), status: http.StatusBadRequest},
		{name: "auth error fails closed", user: &auth.LoggedInUser{Role: auth.USER}, authErr: errors.New("auth unavailable"), status: http.StatusInternalServerError, authCalls: 1},
		{name: "project error fails closed", user: &auth.LoggedInUser{Role: auth.USER}, allowed: true, projectErr: errors.New("projects unavailable"), status: http.StatusInternalServerError, authCalls: 1, projectCalls: 1},
	}
	for _, prefix := range []string{"/logs", "/ws"} {
		for _, tc := range tests {
			t.Run(prefix+"/"+tc.name, func(t *testing.T) {
				e := echo.New()
				a := &projectAccessAuthStub{allowed: tc.allowed, err: tc.authErr}
				p := &projectAccessProjectsStub{belongs: tc.belongs, err: tc.projectErr}
				called := false
				loggedInGroup(e, tc.user).GET(prefix+"/:projectId/:clientId", func(c *echo.Context) error {
					called = true
					return c.NoContent(http.StatusNoContent)
				}, middleware.RequiresProjectAccessMiddleware(a, p))
				project, client := tc.project, tc.client
				if project == "" {
					project = projectID
				}
				if client == "" {
					client = clientID
				}
				req := httptest.NewRequest(http.MethodGet, prefix+"/"+project+"/"+client, nil)
				req.Header.Set("HX-Request", "true")
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				require.Equal(t, tc.status, rec.Code, "Middleware must return the expected status")
				require.Equal(t, tc.redirect, rec.Header().Get("HX-Redirect"), "Denied requests must fully navigate instead of swapping into the private URL")
				require.Empty(t, rec.Header().Get("HX-Location"), "Auth-wall denials must not use HX-Location, which leaves the address bar on the private URL")
				require.Equal(t, tc.status == http.StatusNoContent, called, "Only authorized requests may reach the downstream handler")
				require.Equal(t, tc.authCalls, a.calls, "Authorization must be checked live only after UUID validation")
				require.Equal(t, tc.projectCalls, p.calls, "Client ownership must not be queried before membership is allowed")
				if a.calls > 0 {
					require.Equal(t, tc.user.ID, a.userID, "Authorization must check the logged-in user")
					require.Equal(t, project, a.project, "Authorization must check the requested project")
				}
				if p.calls > 0 {
					require.Equal(t, project, p.project, "Ownership must check the requested project")
					require.Equal(t, client, p.client, "Ownership must check the requested client")
				}
			})
		}
	}
}

func TestDeniedProjectRoutesNeverTouchLogsOrHub(t *testing.T) {
	for _, role := range []auth.Role{auth.USER, auth.ADMIN} {
		for _, deniedByOwnership := range []bool{false, true} {
			for _, path := range []string{
				"/logs/" + projectID + "/" + clientID,
				"/logs/" + projectID + "/" + clientID + "/search",
				"/logs/" + projectID + "/" + clientID + "/instances",
				"/ws/" + projectID + "/" + clientID,
			} {
				t.Run(fmt.Sprintf("%s%s/ownership=%t", role, path, deniedByOwnership), func(t *testing.T) {
					e := echo.New()
					group := loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: role})
					a := &projectAccessAuthStub{allowed: deniedByOwnership}
					var p projects.ProjectsService
					if deniedByOwnership {
						p = &projectAccessProjectsStub{belongs: false}
					}
					// Nil log/hub dependencies prove neither is reached for either denial reason.
					handlers.NewLogsRouter(nil, a, p, group)
					handlers.NewWsConnectionRouter(nil, a, p, group)
					req := httptest.NewRequest(http.MethodGet, path, nil)
					req.Header.Set("HX-Request", "true")
					rec := httptest.NewRecorder()
					e.ServeHTTP(rec, req)
					require.Equal(t, http.StatusOK, rec.Code, "Denied route must respond without invoking nil dependencies")
					require.Equal(t, "/", rec.Header().Get("HX-Redirect"), "Denied route must fully navigate home")
					require.Empty(t, rec.Header().Get("HX-Location"), "Denied route must not swap home into the private URL")
					require.Empty(t, rec.Body.String(), "Denied route must not expose any project data")
				})
			}
		}
	}
}

func TestProjectAccessIsRecheckedOnEveryRequest(t *testing.T) {
	e := echo.New()
	group := loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER, ProjectIDs: []string{projectID}})
	a := &projectAccessAuthStub{allowed: true}
	p := &projectAccessProjectsStub{belongs: true}
	logs := &projectAccessLogsStub{}
	handlers.NewLogsRouter(logs, a, p, group)
	path := "/logs/" + projectID + "/" + clientID + "/instances"
	allowed := httptest.NewRecorder()
	e.ServeHTTP(allowed, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, allowed.Code, "An assigned user must be allowed to query instances")
	require.Contains(t, allowed.Body.String(), "instance", "Authorized requests must reach the log service")

	a.allowed = false
	denied := httptest.NewRecorder()
	e.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusTemporaryRedirect, denied.Code, "A revoked membership must deny the very next non-HTMX request with a full redirect")
	require.Equal(t, "/", denied.Header().Get("Location"), "Non-HTMX denial must preserve the home redirect")
	require.Equal(t, 2, a.calls, "Membership must be checked again rather than trusting cached project IDs")
	require.Equal(t, 1, p.calls, "Revoked membership must deny before querying clients")
	require.Equal(t, 1, logs.calls, "Revoked membership must deny before querying logs")
}

func TestMissingProjectOrClientIDIsRejectedBeforeQueries(t *testing.T) {
	for _, ids := range [][2]string{{"", clientID}, {projectID, ""}} {
		t.Run(fmt.Sprint(ids), func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
			c.Set(auth.UserKey.String(), &auth.LoggedInUser{Role: auth.ADMIN})
			c.SetPathValues(echo.PathValues{
				{Name: "projectId", Value: ids[0]},
				{Name: "clientId", Value: ids[1]},
			})
			err := middleware.RequiresProjectAccessMiddleware(nil, nil)(func(*echo.Context) error {
				t.Fatal("Missing IDs must not reach the downstream handler")
				return nil
			})(c)
			require.NoError(t, err, "Invalid IDs must be handled without querying nil services")
			require.Equal(t, http.StatusBadRequest, rec.Code, "Both route IDs are required")
		})
	}
}

func TestHomepageLoadsOnlyVisibleProjects(t *testing.T) {
	for _, role := range []auth.Role{auth.USER, auth.ADMIN} {
		t.Run(string(role), func(t *testing.T) {
			e := echo.New()
			p := &projectAccessProjectsStub{visible: []projects.Project{{ID: uuid.MustParse(projectID), Name: "Assigned project", Clients: []projects.Client{{ID: uuid.MustParse(clientID), Name: "Friendly client"}}}}}
			handlers.NewHomeHandler(loggedInGroup(e, &auth.LoggedInUser{Role: role, ProjectIDs: []string{projectID}}), p)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, http.StatusOK, rec.Code, "Homepage must render visible projects")
			require.Contains(t, rec.Body.String(), "/logs/"+projectID+"/"+clientID, "Client links must use stable UUIDs")
			require.Contains(t, rec.Body.String(), "Friendly client", "Client cards must display client names")
			if role == auth.ADMIN {
				require.Equal(t, 1, p.allCalls, "Admins must load all projects")
				require.Zero(t, p.assignedCalls, "Admins must not use filtered memberships")
			} else {
				require.Zero(t, p.allCalls, "Regular users must never load all projects")
				require.Equal(t, []string{projectID}, p.assignedIDs, "Regular users must load only assigned project IDs")
			}
		})
	}
	t.Run("empty memberships skip nil project service", func(t *testing.T) {
		e := echo.New()
		handlers.NewHomeHandler(loggedInGroup(e, &auth.LoggedInUser{Role: auth.USER}), nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, http.StatusOK, rec.Code, "Users without assignments must load home without querying projects")
	})
}

func TestDeleteProjectWithClientsReturnsConflict(t *testing.T) {
	e := echo.New()
	p := &projectAccessProjectsStub{deleteErr: errors.New(projects.ErrProjectHasClients)}
	handlers.NewProjectsRouter(p, serverauth.NatsCredentialService{}, e.Group("/admin"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/admin/projects/"+projectID, nil))
	require.Equal(t, http.StatusConflict, rec.Code, "Projects with historical clients must return conflict, not internal server error")
	require.Contains(t, rec.Body.String(), "Cannot delete a project with clients", "Conflict must explain why deletion is restricted")
	require.Equal(t, "none", rec.Header().Get("HX-Reswap"), "A deletion conflict must leave the project row intact")
	require.Contains(t, rec.Header().Get("HX-Trigger"), "Cannot delete a project with clients", "A deletion conflict must also notify the user with a toast")
}
