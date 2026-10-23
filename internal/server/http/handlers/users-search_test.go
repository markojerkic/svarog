package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

type usersSearchAuthStub struct {
	auth.AuthService
	query types.GetUserPageInput
	users []auth.User
}

func (s *usersSearchAuthStub) GetUserPage(_ context.Context, query types.GetUserPageInput) ([]auth.User, int64, error) {
	s.query = query
	return s.users, int64(len(s.users)), nil
}

func TestUsersSearchKeepsTheUsersTable(t *testing.T) {
	user := auth.User{
		ID: uuid.New(), Username: "qa-demo-check", FirstName: "Quality", LastName: "Demo", Role: auth.USER,
	}
	stub := &usersSearchAuthStub{users: []auth.User{user}}
	e := echo.New()
	e.Validator = assignmentValidator{}
	handlers.NewAuthRouter(stub, nil, e.Group("/admin"), e.Group(""), e.Group(""))

	matched := httptest.NewRecorder()
	e.ServeHTTP(matched, httptest.NewRequest(http.MethodGet, "/admin/users?search=qa-demo-check", nil))
	require.Equal(t, http.StatusOK, matched.Code, "User search must render the users page")
	require.Equal(t, "qa-demo-check", stub.query.Search, "User search must filter by the submitted query")
	body := matched.Body.String()
	require.Contains(t, body, `hx-get="/admin/users"`, "Users search must request the users page")
	require.NotContains(t, body, `hx-get="/admin/projects"`, "Users search must not target the projects endpoint")
	require.Contains(t, body, "Username", "User search must keep the users table columns")
	require.Contains(t, body, "qa-demo-check", "A matching user must appear in the users table")
	require.Contains(t, body, `value="qa-demo-check"`, "The search field must keep the query after render")
	require.Contains(t, body, "Delete user", "Row actions must belong to users")
	require.NotContains(t, body, "Project name", "User search must not render the projects table")

	stub.users = nil
	empty := httptest.NewRecorder()
	e.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/admin/users?search=missing-user", nil))
	require.Equal(t, http.StatusOK, empty.Code, "A search with no matches must still render the users page")
	require.Contains(t, empty.Body.String(), "Username", "An empty user search must keep the users table")
	require.NotContains(t, empty.Body.String(), "Project name", "An empty user search must not fall back to projects")
	require.NotContains(t, empty.Body.String(), "qa-demo-check", "An unmatched query must not list unrelated users")

	stub.users = []auth.User{user}
	cleared := httptest.NewRecorder()
	e.ServeHTTP(cleared, httptest.NewRequest(http.MethodGet, "/admin/users?search=", nil))
	require.Equal(t, "", stub.query.Search, "Clearing search must request an unfiltered user page")
	require.Contains(t, cleared.Body.String(), "qa-demo-check", "Clearing search must restore users, not projects")
	require.NotContains(t, cleared.Body.String(), "Project name", "Clearing search must not show the projects table")
}
