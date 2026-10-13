package projects

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	persistence "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
)

type SearchProjectsRequest struct {
	Search string `json:"search" query:"search"`
	Page   int64  `json:"page" query:"page" default:"0" validate:"min=0"`
	Size   int64  `json:"size" query:"size" default:"10" validate:"min=0,max=100"`
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
	DeleteProject(ctx context.Context, id string) error
	ProjectExists(ctx context.Context, projectID, clientID string) bool
	ClientBelongsToProject(ctx context.Context, projectID, clientID string) (bool, error)
}

type PostgresProjectsService struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

const (
	ErrProjectNotFound   = "project not found"
	ErrClientNotFound    = "client not found"
	ErrProjectExists     = "project already exists"
	ErrProjectHasClients = "project has clients; deactivate clients to preserve history"
)

var _ ProjectsService = (*PostgresProjectsService)(nil)

func NewPostgresProjectsService(pool *pgxpool.Pool) *PostgresProjectsService {
	return &PostgresProjectsService{pool: pool, queries: sqlc.New(pool)}
}

func (p *PostgresProjectsService) CreateOrUpdateProject(ctx context.Context, form types.CreateProjectForm) (Project, error) {
	if form.ID == "" {
		return p.CreateProject(ctx, form.Name, form.Clients)
	}
	id, err := uuid.Parse(form.ID)
	if err != nil {
		return Project{}, errors.New(ErrProjectNotFound)
	}
	return p.UpdateProject(ctx, id, form.Name, form.Clients)
}

func validateProject(name string, clients []string) error {
	if strings.TrimSpace(name) == "" {
		return types.NewApiError("Project name is required", map[string]string{"name": "Project name is required"})
	}
	for _, client := range clients {
		if strings.TrimSpace(client) == "" {
			return types.NewApiError("Client name is required", map[string]string{"clients": "Client names must not be blank"})
		}
	}
	return nil
}

func (p *PostgresProjectsService) CreateProject(ctx context.Context, name string, clients []string) (Project, error) {
	if err := validateProject(name, clients); err != nil {
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
	if err := validateProject(name, clients); err != nil {
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
	if query.Page < 0 || query.Size <= 0 || query.Size > 100 || query.Page > math.MaxInt32/query.Size {
		return nil, 0, errors.New("Invalid project pagination")
	}
	rows, err := p.queries.ListProjects(ctx, sqlc.ListProjectsParams{
		Search: query.Search, PageSize: int32(query.Size), PageOffset: int32(query.Page * query.Size),
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

func (p *PostgresProjectsService) DeleteProject(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return errors.New(ErrProjectNotFound)
	}
	_, err = p.queries.DeleteProject(ctx, parsed)
	return projectError(err)
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
	return projects, nil
}

func projectError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New(ErrProjectNotFound)
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		if pgError.Code == "23505" && pgError.ConstraintName == "projects_name_key" {
			return errors.New(ErrProjectExists)
		}
		if pgError.Code == "23001" && pgError.ConstraintName == "clients_project_id_fkey" {
			return errors.New(ErrProjectHasClients)
		}
	}
	return err
}
