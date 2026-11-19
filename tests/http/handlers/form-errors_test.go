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
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/validation"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/stretchr/testify/require"
)

type invalidAssignmentAuthStub struct{}

func (invalidAssignmentAuthStub) CreateUser(context.Context, auth.UserInput) (auth.User, string, error) {
	return auth.User{}, "", auth.ErrProjectSelectionMissing
}

func (invalidAssignmentAuthStub) UpdateUser(context.Context, uuid.UUID, auth.UserInput) (auth.User, error) {
	return auth.User{}, auth.ErrProjectSelectionMissing
}

type invalidProjectFormStub struct{}

func (invalidProjectFormStub) Create(context.Context, projects.ProjectInput) (projects.Project, error) {
	return projects.Project{}, &validation.Error{Key: "validation.failed", Fields: map[string]validation.Violation{"clients": {Key: "projects.clients_blank"}}}
}

func (s invalidProjectFormStub) Update(ctx context.Context, _ uuid.UUID, input projects.ProjectInput) (projects.Project, error) {
	return s.Create(ctx, input)
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
			stub := invalidAssignmentAuthStub{}
			handlers.NewAuthRouter(userRoutesStub{create: stub.CreateUser, update: stub.UpdateUser}, &assignmentProjectsStub{}, e.Group("/admin"))
			projectID := uuid.NewString()
			form := url.Values{"id": {id}, "username": {"username"}, "firstName": {"First"}, "lastName": {"Last"}, "role": {"user"}, "projects": {projectID}}
			req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, "Invalid assignments must return a field validation error")
			require.Contains(t, rec.Body.String(), `id="`+formID+`"`, "Re-rendered form must retain its create/edit identity for the dialog's save button")
			require.Contains(t, rec.Body.String(), html.EscapeString("#"+formID+" [name='projects']"), "Assignment retries must read the failing form's input, not another open form")
			require.Contains(t, rec.Body.String(), "One or more selected projects do not exist", "Assignment errors must identify the projects field")
			require.Equal(t, []assignmentOption{{ID: projectID, Name: projectID, Selected: true}}, projectAssignmentOptions(rec.Body.String()), "Re-rendered validation error must preserve submitted project selection")
		})
	}
	t.Run("malformed identifiers produce localized validation", func(t *testing.T) {
		e := echo.New()
		e.Validator = assignmentValidator{}
		service := auth.NewPostgresAuthService(nil, nil)
		handlers.NewAuthRouter(service, &assignmentProjectsStub{}, e.Group("/admin"))
		form := url.Values{"id": {"not-a-uuid"}, "username": {"username"}, "firstName": {"First"}, "lastName": {"Last"}, "role": {"user"}, "projects": {"also-not-a-uuid"}}
		req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
		req = req.WithContext(i18n.WithLanguage(req.Context(), i18n.Croatian))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, "Malformed user and project identifiers must be rejected before persistence")
		require.Contains(t, rec.Body.String(), "Podaci nisu ispravni.", "Form validation must use the request language")
		require.Contains(t, rec.Body.String(), "Odaberite valjane projekte.", "Invalid project IDs must identify the projects field in the request language")
	})
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
			stub := invalidProjectFormStub{}
			handlers.NewProjectsRouter(projectRoutesStub{create: stub.Create, update: stub.Update}, nil, e.Group("/admin"))
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
