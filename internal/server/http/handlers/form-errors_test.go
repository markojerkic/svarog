package handlers_test

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

type invalidAssignmentAuthStub struct{ auth.AuthService }

func (invalidAssignmentAuthStub) CreateOrUpdateUser(context.Context, types.CreateUserForm) (auth.User, error) {
	return auth.User{}, types.NewApiError("Project not found", map[string]string{"projects": "Select valid projects"})
}

type invalidProjectFormStub struct{ projects.ProjectsService }

func (invalidProjectFormStub) CreateOrUpdateProject(context.Context, types.CreateProjectForm) (projects.Project, error) {
	return projects.Project{}, types.NewApiError("Client name is required", map[string]string{"clients": "Client names must not be blank"})
}

func TestUserAssignmentErrorsPreserveFormIdentity(t *testing.T) {
	for _, id := range []string{"", uuid.NewString()} {
		formID := "new-user-form"
		if id != "" {
			formID = "edit-user-form"
		}
		t.Run(formID, func(t *testing.T) {
			e := echo.New()
			e.Validator = assignmentValidator{}
			handlers.NewAuthRouter(invalidAssignmentAuthStub{}, nil, e.Group("/admin"), e.Group(""), e.Group(""))
			projectID := uuid.NewString()
			form := url.Values{"id": {id}, "username": {"username"}, "firstName": {"First"}, "lastName": {"Last"}, "role": {"user"}, "projects": {projectID}}
			req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, "Invalid assignments must return a field validation error")
			require.Contains(t, rec.Body.String(), `id="`+formID+`"`, "Re-rendered form must retain its create/edit identity for the dialog's save button")
			require.Contains(t, rec.Body.String(), `hx-include="`+html.EscapeString("#"+formID+" [name='projects']")+`"`, "Assignment retries must read the failing form's input, not another open form")
			require.Contains(t, rec.Body.String(), "Select valid projects", "Assignment errors must identify the projects field")
			require.Equal(t, []assignmentOption{{ID: projectID, Name: projectID, Selected: true}}, projectAssignmentOptions(rec.Body.String()), "Re-rendered validation error must preserve submitted project selection")
		})
	}
}

func TestProjectClientErrorsPreserveFormIdentity(t *testing.T) {
	for _, id := range []string{"", uuid.NewString()} {
		formID := "new-project-form"
		if id != "" {
			formID = "edit-project-form"
		}
		t.Run(formID, func(t *testing.T) {
			e := echo.New()
			e.Validator = assignmentValidator{}
			handlers.NewProjectsRouter(invalidProjectFormStub{}, serverauth.NatsCredentialService{}, e.Group("/admin"))
			form := url.Values{"id": {id}, "name": {"Project"}, "clients": {" "}}
			req := httptest.NewRequest(http.MethodPost, "/admin/projects", strings.NewReader(form.Encode()))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, "Invalid client names must be reported as field validation errors, not server failures")
			require.Contains(t, rec.Body.String(), `id="`+formID+`"`, "Client validation error must retain the dialog's create/edit form identity")
			require.Contains(t, rec.Body.String(), "Client names must not be blank", "Client validation error must explain the rejected field")
		})
	}
}
