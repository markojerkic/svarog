package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

type projectCRUDStub struct {
	projects.ProjectsService
	saved      projects.Project
	rows       []projects.Project
	count      int64
	queries    []types.GetProjectPageInput
	writeCalls int
	saveError  error
}

func (s *projectCRUDStub) CreateOrUpdateProject(context.Context, types.CreateProjectForm) (projects.Project, error) {
	s.writeCalls++
	return s.saved, s.saveError
}

func (s *projectCRUDStub) DeleteProject(context.Context, string) error {
	s.writeCalls++
	return nil
}

func (s *projectCRUDStub) GetProjectPage(_ context.Context, query types.GetProjectPageInput) ([]projects.Project, int64, error) {
	s.queries = append(s.queries, query)
	if query.Page*query.Size >= s.count {
		return nil, s.count, nil
	}
	return s.rows, s.count, nil
}

func projectCRUDRequest(t *testing.T, service *projectCRUDStub, method, path, view string, form url.Values, hx bool) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Validator = assignmentValidator{}
	handlers.NewProjectsRouter(service, serverauth.NatsCredentialService{}, e.Group("/admin"))
	req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	req = req.WithContext(i18n.WithLanguage(req.Context(), i18n.Croatian))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	if hx {
		req.Header.Set("HX-Request", "true")
		req.Header.Set("HX-Current-URL", view)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestProjectWritesReloadCurrentFilteredPageInsteadOfAppendingRows(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "edit"}[edit], func(t *testing.T) {
			matching := projects.Project{ID: uuid.New(), Name: "Čć traženi projekt"}
			changed := projects.Project{ID: uuid.New(), Name: "Drugi projekt"}
			service := &projectCRUDStub{saved: changed, rows: []projects.Project{matching}, count: 12}
			form := url.Values{"name": {changed.Name}, "clients": {"api"}}
			if edit {
				form.Set("id", changed.ID.String())
			}
			query := url.Values{"search": {"Čć"}, "page": {"2"}, "size": {"5"}}
			rec := projectCRUDRequest(t, service, http.MethodPost, "/admin/projects", "http://localhost:1323/admin/projects?"+query.Encode(), form, true)
			require.Equal(t, http.StatusOK, rec.Code, "A successful project write must render the refreshed view")
			require.Equal(t, []types.GetProjectPageInput{{Search: "Čć", Page: 2, Size: 5}}, service.queries, "Writes must preserve the current search, page and page size")
			require.Contains(t, rec.Body.String(), `data-project-id="`+matching.ID.String()+`"`, "The response must contain the service's current filtered rows")
			require.NotContains(t, rec.Body.String(), `data-project-id="`+changed.ID.String()+`"`, "An out-of-filter created or renamed project must not be inserted into the table")
			require.Contains(t, rec.Body.String(), `id="new-project-form"`, "Successful writes must supply a fresh creation form")
			require.NotContains(t, rec.Body.String(), `value="`+changed.Name+`"`, "The next creation form must not inherit values from the saved project")
			require.Equal(t, "body", rec.Header().Get("HX-Retarget"), "The complete project view must be replaced so dialogs, rows and totals agree")
			require.Equal(t, "body", rec.Header().Get("HX-Reselect"), "The client must select the new body instead of appending a row")
		})
	}
}

func TestProjectDeletionClampsTheLastPageAndRefreshesItsRows(t *testing.T) {
	remaining := projects.Project{ID: uuid.New(), Name: "QA remaining"}
	service := &projectCRUDStub{rows: []projects.Project{remaining}, count: 1}
	rec := projectCRUDRequest(t, service, http.MethodDelete, "/admin/projects/"+uuid.NewString(), "http://localhost/admin/projects?search=QA&page=1&size=1", nil, true)
	require.Equal(t, http.StatusOK, rec.Code, "Deleting the last row of a page must return a usable refreshed view")
	require.Equal(t, []types.GetProjectPageInput{{Search: "QA", Page: 1, Size: 1}, {Search: "QA", Page: 0, Size: 1}}, service.queries, "An emptied last page must fall back to the nearest existing page")
	require.Equal(t, "/admin/projects?search=QA&size=1", rec.Header().Get("HX-Replace-Url"), "The visible URL must follow the clamped page without losing search or size")
	require.Contains(t, rec.Body.String(), `data-project-id="`+remaining.ID.String()+`"`, "The fallback page must contain its remaining row")
}

func TestProjectInvalidViewDoesNotCommitWrites(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		service := &projectCRUDStub{}
		path := "/admin/projects"
		if method == http.MethodDelete {
			path += "/" + uuid.NewString()
		}
		rec := projectCRUDRequest(t, service, method, path, "http://localhost/admin/projects?page=9223372036854775807", url.Values{"name": {"Projekt"}}, true)
		require.Equal(t, http.StatusBadRequest, rec.Code, "An invalid view must fail before committing a project mutation")
		require.Zero(t, service.writeCalls, "An overflowing page must not partially commit a mutation")
	}
}

func TestProjectInvalidFormAndMissingEditReturnClientErrors(t *testing.T) {
	for _, tc := range []struct {
		form url.Values
		err  error
		code int
	}{
		{form: url.Values{"name": {"Projekt"}, "id": {"not-a-uuid"}}, code: http.StatusBadRequest},
		{form: url.Values{"name": {"Projekt\x00"}}, code: http.StatusBadRequest},
		{form: url.Values{"name": {strings.Repeat("Ž", 256)}}, code: http.StatusBadRequest},
		{form: url.Values{"name": {"Projekt"}, "id": {uuid.NewString()}}, err: errors.New(projects.ErrProjectNotFound), code: http.StatusNotFound},
	} {
		service := &projectCRUDStub{saveError: tc.err}
		rec := projectCRUDRequest(t, service, http.MethodPost, "/admin/projects", "", tc.form, true)
		require.Equal(t, tc.code, rec.Code, "Invalid IDs and text must be client errors; nonexistent edited projects must be 404")
		if tc.code == http.StatusBadRequest {
			require.Zero(t, service.writeCalls, "Invalid project fields must not reach persistence")
		}
		require.Equal(t, "text/html; charset=UTF-8", rec.Header().Get("Content-Type"), "Form failures must remain renderable HTML rather than raw JSON")
	}
}

func TestInvalidProjectListRequestsRenderLocalizedHTMLOrKeepTheHTMXView(t *testing.T) {
	for _, query := range []string{"page=abc", "page=-1", "page=9223372036854775807", "search=%00"} {
		for _, hx := range []bool{false, true} {
			service := &projectCRUDStub{}
			rec := projectCRUDRequest(t, service, http.MethodGet, "/admin/projects?"+query, "", nil, hx)
			require.Equal(t, http.StatusBadRequest, rec.Code, "Malformed project list parameters must not cause server errors")
			require.Empty(t, service.queries, "Malformed parameters must be rejected before querying persistence")
			if hx {
				require.Equal(t, "none", rec.Header().Get("HX-Reswap"), "Failed searches must preserve the existing project table")
				require.NotEmpty(t, rec.Header().Get("HX-Trigger"), "Failed HTMX searches must provide a user-visible error")
			} else {
				require.Contains(t, rec.Body.String(), "Neispravan zahtjev", "Direct navigation must show the localized HTML error page")
			}
		}
	}
}
