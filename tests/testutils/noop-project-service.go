package testutils

import (
	"context"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/projects"
)

type NoopProjectService struct{}

// CreateProject implements [projects.ProjectsService].
func (n *NoopProjectService) Create(ctx context.Context, input projects.ProjectInput) (projects.Project, error) {
	panic("unimplemented")
}

// DeleteProject implements [projects.ProjectsService].
func (n *NoopProjectService) Delete(ctx context.Context, id, confirmationName string) error {
	panic("unimplemented")
}

// GetProject implements [projects.ProjectsService].
func (n *NoopProjectService) Get(ctx context.Context, id string) (projects.Project, error) {
	panic("unimplemented")
}

// GetProjects implements [projects.ProjectsService].
func (n *NoopProjectService) All(ctx context.Context) ([]projects.Project, error) {
	panic("unimplemented")
}

// GetProjectPage implements [projects.ProjectsService].
func (n *NoopProjectService) Page(ctx context.Context, query projects.PageQuery) ([]projects.Project, int64, error) {
	panic("unimplemented")
}

// GetProjectsByIds implements [projects.ProjectsService].
func (n *NoopProjectService) ByIDs(ctx context.Context, ids []string) ([]projects.Project, error) {
	return []projects.Project{}, nil
}

// ProjectExists implements [projects.ProjectsService].
func (n *NoopProjectService) ProjectExists(ctx context.Context, projectId string, clientId string) bool {
	return true
}

func (n *NoopProjectService) ClientBelongsToProject(ctx context.Context, projectID string, clientID string) (bool, error) {
	return true, nil
}

// UpdateProject implements [projects.ProjectsService].
func (n *NoopProjectService) Update(ctx context.Context, id uuid.UUID, input projects.ProjectInput) (projects.Project, error) {
	panic("unimplemented")
}

var _ projects.ProjectsService = &NoopProjectService{}
