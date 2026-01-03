package testutils

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type NoopProjectService struct{}

// CreateOrUpdateProject implements [projects.ProjectsService].
func (n *NoopProjectService) CreateOrUpdateProject(ctx context.Context, project types.CreateProjectForm) (projects.Project, error) {
	panic("unimplemented")
}

// CreateProject implements [projects.ProjectsService].
func (n *NoopProjectService) CreateProject(ctx context.Context, name string, clients []string) (projects.Project, error) {
	panic("unimplemented")
}

// DeleteProject implements [projects.ProjectsService].
func (n *NoopProjectService) DeleteProject(ctx context.Context, id string) error {
	panic("unimplemented")
}

// GetProject implements [projects.ProjectsService].
func (n *NoopProjectService) GetProject(ctx context.Context, id string) (projects.Project, error) {
	panic("unimplemented")
}

// GetProjects implements [projects.ProjectsService].
func (n *NoopProjectService) GetProjects(ctx context.Context) ([]projects.Project, error) {
	panic("unimplemented")
}

// ProjectExists implements [projects.ProjectsService].
func (n *NoopProjectService) ProjectExists(ctx context.Context, projectId string, clientId string) bool {
	return true
}

// UpdateProject implements [projects.ProjectsService].
func (n *NoopProjectService) UpdateProject(ctx context.Context, id primitive.ObjectID, name string, clients []string) (projects.Project, error) {
	panic("unimplemented")
}

var _ projects.ProjectsService = &NoopProjectService{}
