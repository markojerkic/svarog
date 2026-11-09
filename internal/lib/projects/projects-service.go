package projects

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
)

// ArchiveContentDeleter removes archive files after their rows are gone.
// Delete is idempotent for missing content and must target only this backend.
type ArchiveContentDeleter interface {
	Backend() string
	Delete(ctx context.Context, key string) error
}

type SearchProjectsRequest struct {
	Search string `json:"search" query:"search"`
	Page   int64  `json:"page" query:"page" default:"0" validate:"gte=0"`
	Size   int64  `json:"size" query:"size" default:"10" validate:"gte=1,lte=100"`
}

type ProjectsService interface {
	CreateProject(ctx context.Context, name string, clients []string) (Project, error)
	UpdateProject(ctx context.Context, id uuid.UUID, name string, clients []string) (Project, error)
	CreateOrUpdateProject(ctx context.Context, project types.CreateProjectForm) (Project, error)
	GetProject(ctx context.Context, id string) (Project, error)
	GetProjects(ctx context.Context) ([]Project, error)
	GetProjectsByIds(ctx context.Context, ids []string) ([]Project, error)
	GetProjectPage(ctx context.Context, query types.GetProjectPageInput) ([]Project, int64, error)
	SearchProjects(ctx context.Context, request SearchProjectsRequest) ([]Project, error)
	DeleteProject(ctx context.Context, id, confirmationName string) error
	ProjectExists(ctx context.Context, projectID, clientID string) bool
	ClientBelongsToProject(ctx context.Context, projectID, clientID string) (bool, error)
}

type PostgresProjectsService struct {
	pool     *pgxpool.Pool
	queries  *sqlc.Queries
	archives ArchiveContentDeleter
}

const (
	ErrProjectNotFound                   = "project not found"
	ErrClientNotFound                    = "client not found"
	ErrProjectExists                     = "project already exists"
	ErrProjectHasClients                 = "project has clients; deactivate clients to preserve history"
	ErrProjectArchiveBusy                = "project archive is in progress"
	ErrProjectDeleteConfirmationRequired = "project deletion requires its exact current name"
)

var _ ProjectsService = (*PostgresProjectsService)(nil)

func NewPostgresProjectsService(pool *pgxpool.Pool) *PostgresProjectsService {
	return &PostgresProjectsService{pool: pool, queries: sqlc.New(pool)}
}

// SetArchiveStore wires the store that holds ZIP content. Without it, a project
// that has archive rows cannot be deleted, because the files would be left behind.
func (p *PostgresProjectsService) SetArchiveStore(store ArchiveContentDeleter) {
	p.archives = store
}

func (p *PostgresProjectsService) CreateOrUpdateProject(ctx context.Context, form types.CreateProjectForm) (Project, error) {
	if err := ValidateProjectForm(ctx, form); err != nil {
		return Project{}, err
	}
	if form.ID == "" {
		return p.CreateProject(ctx, form.Name, form.Clients)
	}
	id, err := uuid.Parse(form.ID)
	if err != nil {
		return Project{}, errors.New(ErrProjectNotFound)
	}
	return p.UpdateProject(ctx, id, form.Name, form.Clients)
}

func (p *PostgresProjectsService) CreateProject(ctx context.Context, name string, clients []string) (Project, error) {
	if err := validateProject(ctx, name, clients); err != nil {
		return Project{}, err
	}
	row, err := persistence.CreateProjectWithClients(ctx, p.pool, persistence.ProjectWithClientsInput{
		ID: uuid.New(), Name: name, Clients: clients,
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return p.projectWithClients(ctx, row)
}

func (p *PostgresProjectsService) UpdateProject(ctx context.Context, id uuid.UUID, name string, clients []string) (Project, error) {
	if err := validateProject(ctx, name, clients); err != nil {
		return Project{}, err
	}
	row, err := persistence.UpdateProjectWithClients(ctx, p.pool, persistence.ProjectWithClientsInput{
		ID: id, Name: name, Clients: clients,
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return p.projectWithClients(ctx, row)
}

func (p *PostgresProjectsService) GetProject(ctx context.Context, id string) (Project, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return Project{}, errors.New(ErrProjectNotFound)
	}
	row, err := p.queries.GetProjectByID(ctx, parsed)
	if err != nil {
		return Project{}, projectError(err)
	}
	return p.projectWithClients(ctx, row)
}

func (p *PostgresProjectsService) GetProjects(ctx context.Context) ([]Project, error) {
	rows, err := p.queries.ListAllProjects(ctx)
	if err != nil {
		return nil, err
	}
	return p.projectsWithClients(ctx, rows)
}

func (p *PostgresProjectsService) GetProjectsByIds(ctx context.Context, ids []string) ([]Project, error) {
	parsed := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		var err error
		parsed[i], err = uuid.Parse(id)
		if err != nil {
			return nil, errors.New(ErrProjectNotFound)
		}
	}
	rows, err := p.queries.GetProjectsByIDs(ctx, parsed)
	if err != nil {
		return nil, err
	}
	return p.projectsWithClients(ctx, rows)
}

func (p *PostgresProjectsService) GetProjectPage(ctx context.Context, query types.GetProjectPageInput) ([]Project, int64, error) {
	if err := ValidateProjectPage(ctx, query); err != nil {
		return nil, 0, err
	}
	rows, err := p.queries.ListProjects(ctx, sqlc.ListProjectsParams{
		Search:     query.Search,
		PageSize:   query.Size,
		PageOffset: query.Page * query.Size,
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := p.queries.CountProjects(ctx, query.Search)
	if err != nil {
		return nil, 0, err
	}
	projects, err := p.projectsWithClients(ctx, rows)
	return projects, count, err
}

func (p *PostgresProjectsService) SearchProjects(ctx context.Context, request SearchProjectsRequest) ([]Project, error) {
	projects, _, err := p.GetProjectPage(ctx, types.GetProjectPageInput{
		Search: request.Search, Page: request.Page, Size: request.Size,
	})
	return projects, err
}

func (p *PostgresProjectsService) DeleteProject(ctx context.Context, id, confirmationName string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return errors.New(ErrProjectNotFound)
	}
	backend := ""
	if p.archives != nil {
		backend = p.archives.Backend()
	}
	err = persistence.DeleteProjectWithHistory(ctx, p.pool, parsed, confirmationName, backend, func(ctx context.Context, objects []persistence.StoredArchive) error {
		var cleanupErr error
		for _, object := range objects {
			if err := p.archives.Delete(ctx, object.Key); err != nil {
				cleanupErr = errors.Join(cleanupErr, err)
			}
		}
		return cleanupErr
	})
	if errors.Is(err, persistence.ErrArchiveContentCleanup) {
		slog.Error("project deleted but archive files remain", "project", id, "error", err)
		return nil
	}
	return projectDeleteError(err)
}

func (p *PostgresProjectsService) ProjectExists(ctx context.Context, projectID, clientID string) bool {
	project, err := uuid.Parse(projectID)
	if err != nil || project.String() != projectID {
		return false
	}
	if clientID == "*" {
		_, err := p.queries.GetProjectByID(ctx, project)
		return err == nil
	}
	client, err := uuid.Parse(clientID)
	if err != nil || client.String() != clientID {
		return false
	}
	exists, err := p.queries.ActiveProjectClientExists(ctx, sqlc.ActiveProjectClientExistsParams{ProjectID: project, ID: client})
	return err == nil && exists
}

func (p *PostgresProjectsService) ClientBelongsToProject(ctx context.Context, projectID, clientID string) (bool, error) {
	project, err := uuid.Parse(projectID)
	if err != nil {
		return false, nil
	}
	client, err := uuid.Parse(clientID)
	if err != nil {
		return false, nil
	}
	row, err := p.queries.GetClientByID(ctx, client)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil && row.ProjectID == project, err
}

func (p *PostgresProjectsService) projectWithClients(ctx context.Context, row sqlc.Project) (Project, error) {
	projects, err := p.projectsWithClients(ctx, []sqlc.Project{row})
	if err != nil {
		return Project{}, err
	}
	return projects[0], nil
}

func (p *PostgresProjectsService) projectsWithClients(ctx context.Context, rows []sqlc.Project) ([]Project, error) {
	projects := make([]Project, len(rows))
	ids := make([]uuid.UUID, len(rows))
	index := make(map[uuid.UUID]int, len(rows))
	for i, row := range rows {
		projects[i] = Project{ID: row.ID, Name: row.Name, Clients: []Client{}}
		ids[i] = row.ID
		index[row.ID] = i
	}
	if len(rows) == 0 {
		return projects, nil
	}
	clients, err := p.queries.ListActiveClientsForProjects(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, client := range clients {
		i := index[client.ProjectID]
		projects[i].Clients = append(projects[i].Clients, Client{ID: client.ID, Name: client.Name})
	}
	counts, err := p.queries.CountClientsByProject(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, count := range counts {
		projects[index[count.ProjectID]].HasRetainedClients = count.ClientCount > 0
	}
	return projects, nil
}

func projectDeleteError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New(ErrProjectNotFound)
	}
	if errors.Is(err, persistence.ErrProjectArchiveBusy) {
		return errors.New(ErrProjectArchiveBusy)
	}
	if errors.Is(err, persistence.ErrProjectDeleteConfirmationRequired) {
		return errors.New(ErrProjectDeleteConfirmationRequired)
	}
	return projectError(err)
}

func projectError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New(ErrProjectNotFound)
	}
	if persistence.IsConstraintViolation(err, persistence.SQLStateUniqueViolation, "projects_name_key") {
		return errors.New(ErrProjectExists)
	}
	if persistence.IsConstraintViolation(err, persistence.SQLStateRestrictViolation, "clients_project_id_fkey") {
		return errors.New(ErrProjectHasClients)
	}
	return err
}
