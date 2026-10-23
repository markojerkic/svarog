package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

type paginationProjectsStub struct {
	projects.ProjectsService
	pageCalls   int
	searchCalls int
}

func (s *paginationProjectsStub) GetProjectPage(context.Context, types.GetProjectPageInput) ([]projects.Project, int64, error) {
	s.pageCalls++
	return nil, 0, nil
}

func (s *paginationProjectsStub) SearchProjects(context.Context, projects.SearchProjectsRequest) ([]projects.Project, error) {
	s.searchCalls++
	return nil, nil
}

func TestProjectPaginationValidatedBeforeServiceAccess(t *testing.T) {
	for _, path := range []string{"/admin/projects", "/admin/projects/search"} {
		for _, query := range []string{"page=-1&size=10", "size=-1", "size=101"} {
			t.Run(path+"?"+query, func(t *testing.T) {
				service := &paginationProjectsStub{}
				e := echo.New()
				e.Validator = assignmentValidator{}
				handlers.NewProjectsRouter(service, serverauth.NatsCredentialService{}, e.Group("/admin"))

				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+"?"+query, nil))

				require.Equal(t, http.StatusBadRequest, rec.Code, "DTO validation must reject invalid project pagination")
				require.Zero(t, service.pageCalls, "Invalid pagination must not reach project page persistence")
				require.Zero(t, service.searchCalls, "Invalid pagination must not reach project search persistence")
			})
		}
	}
}
