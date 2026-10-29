package handlers_test

import (
	"context"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	serverhttp "github.com/markojerkic/svarog/internal/server/http"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
)

type connStringProjectStub struct {
	projects.ProjectsService
	project projects.Project
}

func (s *connStringProjectStub) GetProject(context.Context, string) (projects.Project, error) {
	return s.project, nil
}

func TestPastConnectionStringExpiryIsAFieldError(t *testing.T) {
	projectID := uuid.New()
	clientID := uuid.New()
	service := &connStringProjectStub{project: projects.Project{
		ID:      projectID,
		Name:    "Logs",
		Clients: []projects.Client{{ID: clientID, Name: "api"}},
	}}
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	for _, lang := range []struct {
		language i18n.Language
		message  string
	}{
		{i18n.English, "Expiry date must be today or later"},
		{i18n.Croatian, "Datum isteka mora biti danas ili kasnije"},
	} {
		t.Run(string(lang.language), func(t *testing.T) {
			e := echo.New()
			e.Validator = serverhttp.NewValidator()
			handlers.NewProjectsRouter(service, *newHandlerCredentialService(t), e.Group("/admin"))

			form := url.Values{
				"projectId": {projectID.String()},
				"clientId":  {clientID.String()},
				"expiry":    {"2000-01-01"},
			}
			req := httptest.NewRequest(http.MethodPost, "/admin/projects/conn-string", strings.NewReader(form.Encode()))
			req = req.WithContext(i18n.WithLanguage(req.Context(), lang.language))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			body := rec.Body.String()
			require.Equal(t, http.StatusBadRequest, rec.Code, "a past expiry must be a validation error, not a generated credential or a server failure")
			require.Contains(t, body, `id="connection-string-form"`, "the past expiry must re-render the connection-string form")
			require.Contains(t, body, lang.message, "the past expiry must be reported on the expiry field in the request language")
			require.Contains(t, body, `data-slot="field-error"`, "the past expiry must be shown as a field error")
			require.Equal(t, "outerHTML", rec.Header().Get("HX-Reswap"), "the past expiry must replace the posted form, matching other validation errors")
			require.Equal(t, "closest *", rec.Header().Get("HX-Retarget"), "the past expiry must retarget the posted form")
			require.Equal(t, "form", rec.Header().Get("HX-Reselect"), "the past expiry response must select the re-rendered form")
			require.NotContains(t, rec.Header().Get("HX-Trigger"), "toast", "a past expiry must not become the generic credentials-generation failure toast")
			require.Contains(t, body, `data-templ-start-month="`+startMonth+`"`, "the calendar must not offer months before the current UTC month")
			require.Contains(t, body, `data-templ-disabled="`+html.EscapeString(`{"before":"`+today+`"}`)+`"`, "the calendar must disable days before today and leave today selectable")
		})
	}
}

func newHandlerCredentialService(t *testing.T) *serverauth.NatsCredentialService {
	t.Helper()
	account, err := nkeys.CreateAccount()
	require.NoError(t, err, "a test account key must be created without a NATS server")
	seed, err := account.Seed()
	require.NoError(t, err, "a test account seed must be readable")
	service, err := serverauth.NewNatsCredentialService(string(seed), "nats.example:4222", &testutils.NoopProjectService{})
	require.NoError(t, err, "credential generation must be constructible without a running NATS server")
	return service
}
