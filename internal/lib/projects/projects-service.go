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
)

// ArchiveContentDeleter removes archive files after their rows are gone.
// Delete is idempotent for missing content and must target only this backend.
type ArchiveContentDeleter interface {
	Backend() string
	Delete(ctx context.Context, key string) error
}

type PostgresProjectsService struct {
	pool     *pgxpool.Pool
	queries  *sqlc.Queries
	archives ArchiveContentDeleter
}

var (
	ErrNotFound             = errors.New("project not found")
	ErrExists               = errors.New("project already exists")
	ErrHasClients           = errors.New("project has clients; deactivate clients to preserve history")
	ErrArchiveBusy          = errors.New("project archive is in progress")
	ErrConfirmationRequired = errors.New("project deletion requires its exact current name")
)

// NewPostgresProjectsService uses archives to remove retained ZIPs after deletion.
func NewPostgresProjectsService(pool *pgxpool.Pool, archives ArchiveContentDeleter) *PostgresProjectsService {
	return &PostgresProjectsService{pool: pool, queries: sqlc.New(pool), archives: archives}
}

func (p *PostgresProjectsService) Create(ctx context.Context, input ProjectInput) (Project, error) {
	if err := validateProject(input); err != nil {
		return Project{}, err
	}
	row, err := persistence.CreateProjectWithClients(ctx, p.pool, persistence.ProjectWithClientsInput{
		ID: uuid.New(), Name: input.Name, Clients: input.Clients,
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return p.projectWithClients(ctx, row)
}

func (p *PostgresProjectsService) Update(ctx context.Context, id uuid.UUID, input ProjectInput) (Project, error) {
	if err := validateProject(input); err != nil {
		return Project{}, err
	}
	row, err := persistence.UpdateProjectWithClients(ctx, p.pool, persistence.ProjectWithClientsInput{
		ID: id, Name: input.Name, Clients: input.Clients,
	})
	if err != nil {
		return Project{}, projectError(err)
	}
	return p.projectWithClients(ctx, row)
}

func (p *PostgresProjectsService) Get(ctx context.Context, id string) (Project, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return Project{}, ErrNotFound
	}
	row, err := p.queries.GetProjectByID(ctx, parsed)
	if err != nil {
		return Project{}, projectError(err)
	}
	return p.projectWithClients(ctx, row)
}

func (p *PostgresProjectsService) All(ctx context.Context) ([]Project, error) {
	rows, err := p.queries.ListAllProjects(ctx)
	if err != nil {
		return nil, err
	}
	return p.projectsWithClients(ctx, rows)
}

func (p *PostgresProjectsService) ByIDs(ctx context.Context, ids []string) ([]Project, error) {
	parsed := make([]uuid.UUID, len(ids))
	for i, id := range ids {
		var err error
		parsed[i], err = uuid.Parse(id)
		if err != nil {
			return nil, ErrNotFound
		}
	}
	rows, err := p.queries.GetProjectsByIDs(ctx, parsed)
	if err != nil {
		return nil, err
	}
	return p.projectsWithClients(ctx, rows)
}

func (p *PostgresProjectsService) Page(ctx context.Context, query PageQuery) ([]Project, int64, error) {
	rows, err := p.queries.ListProjects(ctx, sqlc.ListProjectsParams{
		Search:     query.Search(),
		PageSize:   query.Size(),
		PageOffset: query.Page() * query.Size(),
	})
	if err != nil {
		return nil, 0, err
	}
	count, err := p.queries.CountProjects(ctx, query.Search())
	if err != nil {
		return nil, 0, err
	}
	projects, err := p.projectsWithClients(ctx, rows)
	return projects, count, err
}

func (p *PostgresProjectsService) Delete(ctx context.Context, id, confirmationName string) error {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return ErrNotFound
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
	if errors.Is(err, persistence.ErrProjectArchiveBusy) {
		return ErrArchiveBusy
	}
	if errors.Is(err, persistence.ErrProjectDeleteConfirmationRequired) {
		return ErrConfirmationRequired
	}
	return projectError(err)
}

func projectError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if persistence.IsConstraintViolation(err, persistence.SQLStateUniqueViolation, "projects_name_key") {
		return ErrExists
	}
	if persistence.IsConstraintViolation(err, persistence.SQLStateRestrictViolation, "clients_project_id_fkey") {
		return ErrHasClients
	}
	return err
}
