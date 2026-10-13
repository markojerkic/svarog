package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/stretchr/testify/require"
)

func TestProjectPersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := newPostgresPool(t, ctx)
	q := sqlc.New(pool)
	run := func(name string, test func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE projects, users CASCADE")
			require.NoError(t, err, "clear projects, users and dependent rows before %s", name)
			test(t)
		})
	}
	create := func(t *testing.T, name string, clients ...string) sqlc.Project {
		t.Helper()
		project, err := store.CreateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{
			ID: uuid.New(), Name: name, Clients: clients,
		})
		require.NoError(t, err, "create fixture project %q with clients", name)
		return project
	}

	run("create lookup constraints and client ownership", func(t *testing.T) {
		project := create(t, "Project", "api", "worker", "api")
		byID, err := q.GetProjectByID(ctx, project.ID)
		require.NoError(t, err, "look up project by ID")
		require.Equal(t, project, byID, "ID lookup returns persisted project")
		byName, err := q.GetProjectByName(ctx, project.Name)
		require.NoError(t, err, "look up project by name")
		require.Equal(t, project, byName, "name lookup returns persisted project")
		clients, err := q.ListProjectClients(ctx, project.ID)
		require.NoError(t, err, "list project clients")
		require.Len(t, clients, 2, "deduplicate supplied client names")
		require.Equal(t, "api", clients[0].Name, "sort clients by name")
		client, err := q.GetClientByID(ctx, clients[0].ID)
		require.NoError(t, err, "look up client by ID")
		require.Equal(t, clients[0], client, "client lookup agrees with project client list")
		require.True(t, client.IsActive, "new clients are active")
		require.Equal(t, project.ID, client.ProjectID, "client belongs to creating project")
		require.False(t, client.CreatedAt.IsZero(), "set client creation timestamp")
		_, err = q.CreateProject(ctx, sqlc.CreateProjectParams{ID: uuid.New(), Name: "Project"})
		requireSQLState(t, err, "23505", "reject duplicate project name")
		_, err = q.CreateProject(ctx, sqlc.CreateProjectParams{ID: uuid.New(), Name: "  "})
		requireSQLState(t, err, "23514", "reject blank project name")
		_, err = q.CreateClient(ctx, sqlc.CreateClientParams{ID: uuid.New(), ProjectID: project.ID, Name: "api"})
		requireSQLState(t, err, "23505", "reject duplicate client name within a project")
		_, err = q.CreateClient(ctx, sqlc.CreateClientParams{ID: uuid.New(), ProjectID: uuid.New(), Name: "api"})
		requireSQLState(t, err, "23503", "client foreign key rejects missing parent project")
		other := create(t, "Other", "api")
		exists, err := q.ActiveProjectClientExists(ctx, sqlc.ActiveProjectClientExistsParams{ProjectID: project.ID, ID: client.ID})
		require.NoError(t, err, "check active client in owning project")
		require.True(t, exists, "active client exists in its owning project")
		exists, err = q.ActiveProjectClientExists(ctx, sqlc.ActiveProjectClientExistsParams{ProjectID: other.ID, ID: client.ID})
		require.NoError(t, err, "check client in another project")
		require.False(t, exists, "client does not belong to another project")
		_, err = q.DeactivateClient(ctx, sqlc.DeactivateClientParams{ID: client.ID, ProjectID: other.ID})
		require.ErrorIs(t, err, pgx.ErrNoRows, "another project cannot deactivate this client")
		deactivated, err := q.DeactivateClient(ctx, sqlc.DeactivateClientParams{ID: client.ID, ProjectID: project.ID})
		require.NoError(t, err, "owning project can deactivate its client")
		require.False(t, deactivated.IsActive, "deactivation clears active flag")
	})

	run("literal search stable pagination and selected ids", func(t *testing.T) {
		for _, name := range []string{"alpha", "ALPHA", "Beta", "literal%", "literal_"} {
			create(t, name)
		}
		all, err := q.ListProjects(ctx, sqlc.ListProjectsParams{Search: "", PageSize: 10})
		require.NoError(t, err, "list all projects without search filter")
		require.Len(t, all, 5, "return all fixture projects")
		require.Less(t, all[0].ID.String(), all[1].ID.String(), "break equal case-insensitive project names by ID")
		first, err := q.ListProjects(ctx, sqlc.ListProjectsParams{Search: "", PageSize: 2})
		require.NoError(t, err, "fetch first project page")
		rest, err := q.ListProjects(ctx, sqlc.ListProjectsParams{Search: "", PageSize: 10, PageOffset: 2})
		require.NoError(t, err, "fetch remaining project page")
		require.Equal(t, all, append(first, rest...), "pagination preserves ordering without omissions or duplicates")
		for search, expected := range map[string]int{"": 5, "ALpHa": 2, "%": 1, "_": 1, "missing": 0} {
			rows, err := q.ListProjects(ctx, sqlc.ListProjectsParams{Search: search, PageSize: 10})
			require.NoError(t, err, "list projects matching literal case-insensitive search %q", search)
			count, err := q.CountProjects(ctx, search)
			require.NoError(t, err, "count projects matching search %q", search)
			require.Len(t, rows, expected, "literal case-insensitive search %q returns expected projects", search)
			require.EqualValues(t, expected, count, "count agrees with list for search %q", search)
		}
		selected, err := q.GetProjectsByIDs(ctx, []uuid.UUID{all[2].ID, all[0].ID, all[0].ID, uuid.New()})
		require.NoError(t, err, "fetch selected project IDs")
		require.Equal(t, []sqlc.Project{all[0], all[2]}, selected, "selected projects are sorted, deduplicated and omit missing IDs")
		selected, err = q.GetProjectsByIDs(ctx, []uuid.UUID{})
		require.NoError(t, err, "fetch empty project selection")
		require.Empty(t, selected, "empty ID selection returns no projects")
	})

	run("update deactivates preserves history and reuses ids", func(t *testing.T) {
		project := create(t, "Project", "api", "worker")
		before, err := q.ListProjectClients(ctx, project.ID)
		require.NoError(t, err, "list clients before project update")
		_, err = pool.Exec(ctx, `INSERT INTO logs (client_id, instance_name, occurred_at, sequence_number, message)
			VALUES ($1, 'instance', now(), 0, 'history')`, before[1].ID)
		require.NoError(t, err, "insert historical log for worker client")
		updated, err := store.UpdateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{
			ID: project.ID, Name: "Renamed", Clients: []string{"api", "new", "api"},
		})
		require.NoError(t, err, "update project name and replace clients")
		require.Equal(t, "Renamed", updated.Name, "persist updated project name")
		require.Equal(t, project.CreatedAt, updated.CreatedAt, "update preserves project creation timestamp")
		require.False(t, updated.UpdatedAt.Before(project.UpdatedAt), "update timestamp must not move backwards")
		active, err := q.ListActiveProjectClients(ctx, project.ID)
		require.NoError(t, err, "list active clients after replacement")
		require.Len(t, active, 2, "replacement deduplicates active client names")
		require.Equal(t, before[0].ID, active[0].ID, "unchanged api client retains its ID")
		worker, err := q.GetClientByID(ctx, before[1].ID)
		require.NoError(t, err, "retrieve removed worker client for historical access")
		require.False(t, worker.IsActive, "removed client is deactivated rather than deleted")
		exists, err := q.ActiveProjectClientExists(ctx, sqlc.ActiveProjectClientExistsParams{ProjectID: project.ID, ID: worker.ID})
		require.NoError(t, err, "check deactivated client availability")
		require.False(t, exists, "deactivated client is not an active project client")
		reactivated, err := q.ActivateClientByName(ctx, sqlc.ActivateClientByNameParams{ID: uuid.New(), ProjectID: project.ID, Name: worker.Name})
		require.NoError(t, err, "reactivate existing client by name")
		require.Equal(t, worker.ID, reactivated.ID, "reactivation reuses historical client ID")
		require.Equal(t, worker.CreatedAt, reactivated.CreatedAt, "reactivation preserves creation timestamp")
		require.True(t, reactivated.IsActive, "reactivation sets active flag")
		_, err = store.UpdateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{ID: project.ID, Name: "Renamed"})
		require.NoError(t, err, "replace project clients with empty set")
		active, err = q.ListActiveProjectClients(ctx, project.ID)
		require.NoError(t, err, "list active clients after clearing set")
		require.Empty(t, active, "empty replacement deactivates every client")
		var logs int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM logs WHERE client_id = $1", worker.ID).Scan(&logs), "count historical worker logs")
		require.EqualValues(t, 1, logs, "client deactivation preserves historical logs")
		_, err = q.DeleteProject(ctx, project.ID)
		requireSQLState(t, err, "23001", "historical logs prevent deleting their project")
	})

	run("failed compound writes roll back", func(t *testing.T) {
		id := uuid.New()
		result, err := store.CreateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{ID: id, Name: "Bad", Clients: []string{"valid", " "}})
		requireSQLState(t, err, "23514", "blank client name rejects compound project creation")
		require.Equal(t, sqlc.Project{}, result, "failed creation returns no project")
		_, err = q.GetProjectByID(ctx, id)
		require.ErrorIs(t, err, pgx.ErrNoRows, "invalid client rolls back project creation")
		project := create(t, "Original", "api")
		before, err := q.ListProjectClients(ctx, project.ID)
		require.NoError(t, err, "snapshot clients before invalid replacement")
		_, err = store.UpdateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{
			ID: project.ID, Name: "Changed", Clients: []string{"new", ""},
		})
		requireSQLState(t, err, "23514", "empty client name rejects compound project update")
		after, err := q.GetProjectByID(ctx, project.ID)
		require.NoError(t, err, "reload project after failed update")
		require.Equal(t, project, after, "failed update preserves all project fields")
		clients, err := q.ListProjectClients(ctx, project.ID)
		require.NoError(t, err, "reload clients after failed update")
		require.Equal(t, before, clients, "failed update preserves client set and state")
		_, err = store.UpdateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{ID: uuid.New(), Name: "Missing"})
		require.ErrorIs(t, err, pgx.ErrNoRows, "cannot update missing project")
		create(t, "Taken")
		_, err = store.UpdateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{ID: project.ID, Name: "Taken"})
		requireSQLState(t, err, "23505", "reject renaming project to an existing project name")
	})

	run("concurrent project replacements do not mix client sets", func(t *testing.T) {
		project := create(t, "Original", "old")
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, name := range []string{"First", "Second"} {
			go func() {
				<-start
				_, err := store.UpdateProjectWithClients(ctx, pool, store.ProjectWithClientsInput{
					ID: project.ID, Name: name, Clients: []string{name + "-api", name + "-worker"},
				})
				results <- err
			}()
		}
		close(start)
		for range 2 {
			require.NoError(t, <-results, "each concurrent project replacement succeeds")
		}
		updated, err := q.GetProjectByID(ctx, project.ID)
		require.NoError(t, err, "reload project after concurrent replacements")
		clients, err := q.ListActiveProjectClients(ctx, project.ID)
		require.NoError(t, err, "list clients after concurrent replacements")
		require.Len(t, clients, 2, "concurrent replacements leave one complete client set")
		require.Equal(t, updated.Name+"-api", clients[0].Name, "api client belongs to winning project replacement")
		require.Equal(t, updated.Name+"-worker", clients[1].Name, "worker client belongs to same winning replacement")
	})

	run("concurrent membership replacements do not merge sets", func(t *testing.T) {
		user, err := store.CreateUserWithToken(ctx, pool, authInput("user"))
		require.NoError(t, err, "create user for concurrent membership replacements")
		first := create(t, "First")
		second := create(t, "Second")
		start := make(chan struct{})
		results := make(chan error, 2)
		for _, id := range []uuid.UUID{first.ID, second.ID} {
			go func() {
				<-start
				results <- store.ReplaceUserProjects(ctx, pool, user.ID, []uuid.UUID{id})
			}()
		}
		close(start)
		for range 2 {
			require.NoError(t, <-results, "each concurrent membership replacement succeeds")
		}
		rows, err := q.ListUserProjects(ctx, user.ID)
		require.NoError(t, err, "list memberships after concurrent replacements")
		require.Len(t, rows, 1, "concurrent replacements do not merge membership sets")
		require.Contains(t, []uuid.UUID{first.ID, second.ID}, rows[0].ID, "remaining membership comes from one replacement")
	})

	run("memberships replacement access and cascades", func(t *testing.T) {
		user, err := store.CreateUserWithToken(ctx, pool, authInput("user"))
		require.NoError(t, err, "create regular user for project access checks")
		adminInput := authInput("admin")
		adminInput.Role = "admin"
		admin, err := store.CreateUserWithToken(ctx, pool, adminInput)
		require.NoError(t, err, "create admin for project access checks")
		first := create(t, "First")
		second := create(t, "Second")
		checkAccess := func(userID, projectID uuid.UUID, expected bool, scenario string) {
			t.Helper()
			access, err := q.UserHasProjectAccess(ctx, sqlc.UserHasProjectAccessParams{UserID: userID, ProjectID: projectID})
			require.NoError(t, err, "evaluate access: %s", scenario)
			require.Equal(t, expected, access, scenario)
		}
		checkAccess(user.ID, first.ID, false, "unassigned regular user is denied project access")
		checkAccess(admin.ID, first.ID, true, "admin is allowed access to an existing project without membership")
		checkAccess(admin.ID, uuid.New(), false, "missing project is denied even for an admin")
		checkAccess(uuid.New(), first.ID, false, "missing user is denied access to an existing project")
		require.NoError(t, store.ReplaceUserProjects(ctx, pool, user.ID, []uuid.UUID{first.ID, first.ID, second.ID}), "assign projects with duplicate IDs")
		rows, err := q.ListUserProjects(ctx, user.ID)
		require.NoError(t, err, "list assigned projects")
		require.Equal(t, []sqlc.Project{first, second}, rows, "memberships are sorted and duplicate IDs are deduplicated")
		checkAccess(user.ID, first.ID, true, "assigned regular user is allowed project access")
		err = store.ReplaceUserProjects(ctx, pool, user.ID, []uuid.UUID{second.ID, uuid.New()})
		requireSQLState(t, err, "23503", "membership foreign key rejects missing parent project during replacement")
		rows, err = q.ListUserProjects(ctx, user.ID)
		require.NoError(t, err, "list memberships after failed replacement")
		require.Equal(t, []sqlc.Project{first, second}, rows, "failed replacement preserves original memberships")
		require.NoError(t, q.RemoveUserFromProject(ctx, sqlc.RemoveUserFromProjectParams{UserID: user.ID, ProjectID: first.ID}), "remove first project membership")
		checkAccess(user.ID, first.ID, false, "regular user is denied access after membership removal")
		require.NoError(t, store.ReplaceUserProjects(ctx, pool, user.ID, nil), "clear all user memberships")
		rows, err = q.ListUserProjects(ctx, user.ID)
		require.NoError(t, err, "list memberships after clearing")
		require.Empty(t, rows, "empty replacement removes every membership")
		require.ErrorIs(t, store.ReplaceUserProjects(ctx, pool, uuid.New(), nil), pgx.ErrNoRows, "cannot replace memberships for missing user")
		err = q.AddUserToProject(ctx, sqlc.AddUserToProjectParams{UserID: uuid.New(), ProjectID: first.ID})
		requireSQLState(t, err, "23503", "membership foreign key rejects missing parent user")
		require.NoError(t, store.ReplaceUserProjects(ctx, pool, user.ID, []uuid.UUID{first.ID, second.ID}), "restore memberships before cascade deletion checks")
		deleted, err := q.DeleteProject(ctx, first.ID)
		require.NoError(t, err, "delete project with memberships but no logs")
		require.Equal(t, first.ID, deleted, "delete returns removed project ID")
		_, err = q.DeleteProject(ctx, first.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows, "deleting already removed project returns no rows")
		rows, err = q.ListUserProjects(ctx, user.ID)
		require.NoError(t, err, "list memberships after project deletion")
		require.Equal(t, []sqlc.Project{second}, rows, "project deletion cascades only its own membership")
		_, err = q.DeleteUser(ctx, user.ID)
		require.NoError(t, err, "delete user with remaining membership")
		var count int64
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM user_projects WHERE user_id = $1", user.ID).Scan(&count), "count memberships after user deletion")
		require.Zero(t, count, "user deletion cascades to all memberships")
	})
}
