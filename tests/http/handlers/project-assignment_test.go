package handlers_test

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/combobox"
	usercomponents "github.com/markojerkic/svarog/internal/server/ui/components/users"
	"github.com/stretchr/testify/require"
)

type assignmentValidator struct{}

func (assignmentValidator) Validate(value interface{}) error {
	return validator.New().Struct(value)
}

type assignmentProjectsStub struct {
	projects.ProjectsService
	all         []projects.Project
	loadedIDs   []string
	lookupCalls int
	searchCalls int
}

func (s *assignmentProjectsStub) GetProjectsByIds(_ context.Context, ids []string) ([]projects.Project, error) {
	s.lookupCalls++
	s.loadedIDs = ids
	var selected []projects.Project
	for _, id := range ids {
		for _, project := range s.all {
			if project.ID.String() == id {
				selected = append(selected, project)
			}
		}
	}
	return selected, nil
}

func (s *assignmentProjectsStub) SearchProjects(_ context.Context, request projects.SearchProjectsRequest) ([]projects.Project, error) {
	s.searchCalls++
	var matches []projects.Project
	for _, project := range s.all {
		if strings.Contains(strings.ToLower(project.Name), strings.ToLower(request.Search)) {
			matches = append(matches, project)
		}
	}
	start := int(request.Page * request.Size)
	if start >= len(matches) {
		return nil, nil
	}
	end := min(start+int(request.Size), len(matches))
	return matches[start:end], nil
}

type assignmentAuthStub struct {
	auth.AuthService
	user      auth.User
	submitted types.CreateUserForm
}

func (s *assignmentAuthStub) GetUserByID(context.Context, string) (auth.User, error) {
	return s.user, nil
}

func (s *assignmentAuthStub) CreateOrUpdateUser(_ context.Context, form types.CreateUserForm) (auth.User, error) {
	s.submitted = form
	return s.user, nil
}

type assignmentOption struct {
	ID       string
	Name     string
	Selected bool
}

func projectAssignmentOptions(markup string) []assignmentOption {
	pattern := regexp.MustCompile(`(?s)<div\b[^>]*role="option"[^>]*>.*?</div>`)
	valuePattern := regexp.MustCompile(`data-templ-value="([^"]*)"`)
	namePattern := regexp.MustCompile(`<span>(.*?)</span>`)
	var options []assignmentOption
	for _, tag := range pattern.FindAllString(markup, -1) {
		value := valuePattern.FindStringSubmatch(tag)
		name := namePattern.FindStringSubmatch(tag)
		if value == nil || name == nil {
			continue
		}
		if _, err := uuid.Parse(value[1]); err != nil {
			continue // The user form also contains non-project role options.
		}
		options = append(options, assignmentOption{
			ID:       value[1],
			Selected: strings.Contains(tag, `data-selected="true"`),
			Name:     strings.TrimSpace(html.UnescapeString(name[1])),
		})
	}
	return options
}

func assignmentSearch(t *testing.T, e *echo.Echo, search string, page int, ids []string) *httptest.ResponseRecorder {
	t.Helper()
	query := url.Values{"search": {search}, "page": {strconv.Itoa(page)}, "size": {"2"}, "projects": {strings.Join(ids, ",")}}
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/projects/search?"+query.Encode(), nil))

	require.Equal(t, http.StatusOK, rec.Code, "Project search must render the current selection and search results")
	return rec
}

func TestProjectAssignmentSearchPreservesSelectionsAcrossSearchAndPagination(t *testing.T) {
	alreadyAssigned := projects.Project{
		ID:   uuid.New(),
		Name: "Candidate already assigned",
	}
	outsideSearch := projects.Project{
		ID:   uuid.New(),
		Name: "Outside the search",
	}
	newSelection := projects.Project{
		ID:   uuid.New(),
		Name: "Candidate new selection",
	}
	offPageAssigned := projects.Project{
		ID:   uuid.New(),
		Name: "Candidate off-page assignment",
	}
	laterResult := projects.Project{
		ID:   uuid.New(),
		Name: "Candidate later result",
	}
	p := &assignmentProjectsStub{
		all: []projects.Project{
			alreadyAssigned,
			newSelection,
			offPageAssigned,
			laterResult,
			outsideSearch,
		},
	}
	e := echo.New()
	e.Validator = assignmentValidator{}
	handlers.NewProjectsRouter(p, serverauth.NatsCredentialService{}, e.Group("/admin"))
	selectedIDs := []string{
		alreadyAssigned.ID.String(),
		outsideSearch.ID.String(),
		offPageAssigned.ID.String(),
	}

	first := assignmentSearch(t, e, "Candidate", 0, append(selectedIDs, alreadyAssigned.ID.String()))

	require.Equal(t, selectedIDs, p.loadedIDs, "Selected IDs must be deduplicated before loading their names")
	expectedFirstPage := []assignmentOption{
		{
			ID:       alreadyAssigned.ID.String(),
			Name:     alreadyAssigned.Name,
			Selected: true,
		},
		{
			ID:       outsideSearch.ID.String(),
			Name:     outsideSearch.Name,
			Selected: true,
		},
		{
			ID:       offPageAssigned.ID.String(),
			Name:     offPageAssigned.Name,
			Selected: true,
		},
		{
			ID:   newSelection.ID.String(),
			Name: newSelection.Name,
		},
	}
	require.Equal(t, expectedFirstPage, projectAssignmentOptions(first.Body.String()), "First-page results must pin every named selection, including off-search and off-page projects, without duplicates")

	links := regexp.MustCompile(`hx-get="([^"]*)"`).FindAllStringSubmatch(first.Body.String(), -1)
	require.Len(t, links, 1, "A full raw page must still provide pagination after selected duplicates are excluded")
	next, err := url.Parse(html.UnescapeString(links[0][1]))

	require.NoError(t, err, "Pagination must produce a valid URL")
	require.Equal(t, "1", next.Query().Get("page"), "Pagination must advance the raw search page")
	require.Equal(t, "Candidate", next.Query().Get("search"), "Pagination must retain the current search")
	require.False(t, next.Query().Has("projects"), "Pagination must use the live included input, not stale selections embedded in its URL")

	// Clicking a result marks it selected in the existing DOM before pagination appends more options.
	selectedIDs = append(selectedIDs, newSelection.ID.String())
	second := assignmentSearch(t, e, "Candidate", 1, selectedIDs)

	require.Equal(t, 1, p.lookupCalls, "Appending a later page must not render a second copy of the pinned selections")
	expectedLaterPage := []assignmentOption{
		{
			ID:   laterResult.ID.String(),
			Name: laterResult.Name,
		},
	}
	require.Equal(t, expectedLaterPage, projectAssignmentOptions(second.Body.String()), "Later pages must exclude every currently selected ID, including newly selected and off-page projects")

	changed := assignmentSearch(t, e, "later", 0, selectedIDs)

	options := projectAssignmentOptions(changed.Body.String())
	require.Len(t, options, 5, "Changing search must retain four selections plus the new matching result")
	for i, id := range selectedIDs {
		require.Equal(t, id, options[i].ID, "A selection outside the new search must remain represented exactly once")
		require.True(t, options[i].Selected, "Preserved assignments must remain selected so DOM serialization cannot drop them")
	}
	expectedNewMatch := assignmentOption{
		ID:   laterResult.ID.String(),
		Name: laterResult.Name,
	}
	require.Equal(t, expectedNewMatch, options[4], "New search matches must remain selectable rather than being assigned automatically")

	cleared := assignmentSearch(t, e, "later", 0, nil)

	expectedClearedSearch := []assignmentOption{
		{
			ID:   laterResult.ID.String(),
			Name: laterResult.Name,
		},
	}
	require.Equal(t, expectedClearedSearch, projectAssignmentOptions(cleared.Body.String()), "An empty live selection must not resurrect previously selected assignments")
	require.Equal(t, 2, p.lookupCalls, "An empty selection must skip selected-project lookups")
}

func TestProjectAssignmentSearchRejectsInvalidSelectedIDsBeforeQueries(t *testing.T) {
	for _, id := range []string{"*", "not-a-uuid", uuid.Nil.String()} {
		t.Run(id, func(t *testing.T) {
			p := &assignmentProjectsStub{}
			e := echo.New()
			e.Validator = assignmentValidator{}
			handlers.NewProjectsRouter(p, serverauth.NatsCredentialService{}, e.Group("/admin"))
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/projects/search?projects="+url.QueryEscape(id), nil))

			require.Equal(t, http.StatusBadRequest, rec.Code, "Malformed selection IDs must be rejected as bad input")
			require.Zero(t, p.lookupCalls, "Malformed selection IDs must not reach selected-project persistence")
			require.Zero(t, p.searchCalls, "Malformed selection IDs must not reach project search")
		})
	}
}

func TestCreateAndEditAssignmentFormsIncludeLiveSelectionInRequests(t *testing.T) {
	project := projects.Project{
		ID:   uuid.New(),
		Name: "Friendly initial project",
	}
	p := &assignmentProjectsStub{all: []projects.Project{project}}
	a := &assignmentAuthStub{
		user: auth.User{
			ID:         uuid.New(),
			Role:       auth.USER,
			ProjectIDs: []string{project.ID.String()},
		},
	}
	e := echo.New()
	e.Validator = assignmentValidator{}
	handlers.NewAuthRouter(a, p, e.Group("/admin"), e.Group(""), e.Group(""))
	edit := httptest.NewRecorder()

	e.ServeHTTP(edit, httptest.NewRequest(http.MethodGet, "/admin/users/"+a.user.ID.String()+"/edit", nil))

	require.Equal(t, http.StatusOK, edit.Code, "The edit form must render named current assignments")
	expectedAssignments := []assignmentOption{
		{
			ID:       project.ID.String(),
			Name:     project.Name,
			Selected: true,
		},
	}
	require.Equal(t, expectedAssignments, projectAssignmentOptions(edit.Body.String()), "The edit form must initialize selection state with stable IDs and friendly names")

	for _, formID := range []string{"new-user-form", "edit-user-form"} {
		t.Run(formID, func(t *testing.T) {
			var rendered bytes.Buffer

			err := usercomponents.NewUserForm(usercomponents.NewUserFormProps{
				FormID: formID,
				Value: types.CreateUserForm{
					Projects: []combobox.Option{
						{
							Value: project.ID.String(),
							Name:  project.Name,
						},
					},
				},
			}).Render(context.Background(), &rendered)

			require.NoError(t, err, "Both create and edit forms must render the assignment control")
			require.Contains(t, rendered.String(), html.EscapeString(fmt.Sprintf("#%s [name='projects']", formID)), "Search requests must include this form's live project input even after the popover is portaled")
			require.Contains(t, rendered.String(), `hx-sync:inherited="this:replace"`, "Search and pagination must share a queue so newer searches supersede stale requests")
			require.Contains(t, rendered.String(), html.EscapeString(fmt.Sprintf("change from:\"#%s [name='projects']\"", formID)), "Changing an assignment must refresh with its live IDs using an HTMX 4 quoted selector")
			require.Contains(t, rendered.String(), `name="projects"`, "Both forms must submit the live selection under the existing projects form field")
			require.Equal(t, expectedAssignments, projectAssignmentOptions(rendered.String()), "Initial selected options must display project names, not UUID labels")
			require.Contains(t, rendered.String(), `data-slot="combobox-chip"`, "The closed picker must show a chip for each assigned project")
			require.Regexp(t, regexp.MustCompile(`data-slot="combobox-chip"[^>]*>\s*`+regexp.QuoteMeta(html.EscapeString(project.Name))), rendered.String(), "The closed chip must show the project name, not only the open-list checkmark")
		})
	}

	for _, formID := range []string{"", a.user.ID.String()} {
		form := url.Values{"id": {formID}, "username": {"username"}, "firstName": {"First"}, "lastName": {"Last"}, "role": {"user"}, "projects": {""}}
		req := httptest.NewRequest(http.MethodPost, "/admin/users", strings.NewReader(form.Encode()))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "Clearing all assignments must still allow creating or updating the user")
		require.Empty(t, a.submitted.ProjectIDs, "Clearing all assignments must submit an empty membership set rather than restore initial IDs")
	}
}
