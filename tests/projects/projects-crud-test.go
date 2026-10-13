package projects

import (
	"context"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (p *ProjectsSuite) TestCreateProject() {
	t := p.Suite.T()

	testCases := []struct {
		name    string
		clients []string
		wantErr bool
	}{
		{
			name:    "test1",
			clients: []string{"test client"},
			wantErr: false,
		},
		{
			// Project name already exists
			name:    "test1",
			clients: []string{"test client"},
			wantErr: true,
		},
		{
			name: "test2",
			// Client name already exists
			clients: []string{"test client", "test client"},
			wantErr: false,
		},
		{
			name:    "test3",
			clients: []string{"test client", "test client2"},
			wantErr: false,
		},
	}

	for i, tc := range testCases {
		project, err := p.ProjectsService.CreateProject(context.Background(), tc.name, tc.clients)
		if tc.wantErr {
			assert.Error(t, err, "Test case %d failed", i)
		} else {
			require.NoError(t, err, "creating project %s should succeed", tc.name)
			assert.NotEqual(t, uuid.Nil, project.ID, "project %s should have a UUID", tc.name)
			expectedNames := tc.clients
			if tc.name == "test2" {
				expectedNames = []string{"test client"}
			}
			assert.ElementsMatch(t, expectedNames, project.ClientNames(), "project %s should contain unique client names", tc.name)
			for _, client := range project.Clients {
				assert.NotEqual(t, uuid.Nil, client.ID, "client %s should have a UUID distinct from its display name", client.Name)
			}
		}
	}

}

func (p *ProjectsSuite) TestGetProject() {
	t := p.Suite.T()

	project, err := p.ProjectsService.CreateProject(context.Background(), "test1", []string{"test client"})
	if err != nil {
		t.Fatalf("Could not create project: %s", err)
	}

	testCases := []struct {
		name    string
		id      string
		clients []string
		wantErr bool
	}{
		{
			name:    "test1",
			id:      project.ID.String(),
			clients: []string{"test client"},
			wantErr: false,
		},
		{
			name:    "test2",
			id:      uuid.NewString(),
			clients: []string{"test client", "test client2"},
			wantErr: true,
		},
	}

	for i, tc := range testCases {
		_, err = p.ProjectsService.GetProject(context.Background(), tc.id)
		if tc.wantErr {
			assert.Error(t, err, "Test case %d failed", i)
			if err != nil {
				assert.Equal(t, projects.ErrProjectNotFound, err.Error(), "Test case %d failed", i)
			}
		} else {
			assert.NoError(t, err, "Test case %d failed", i)
		}
	}
}

func (p *ProjectsSuite) TestDeleteProject() {
	t := p.Suite.T()

	project, err := p.ProjectsService.CreateProject(context.Background(), "test1", nil)
	if err != nil {
		t.Fatalf("Could not create project: %s", err)
	}

	err = p.ProjectsService.DeleteProject(context.Background(), project.ID.String())
	require.NoError(t, err, "deleting a project without clients should succeed")

	_, err = p.ProjectsService.GetProject(context.Background(), project.ID.String())
	assert.EqualError(t, err, projects.ErrProjectNotFound, "deleted project should no longer exist")
}

func (p *ProjectsSuite) TestDeleteProject_CleansUpUserReferences() {
	t := p.Suite.T()
	ctx := context.Background()

	project, err := p.ProjectsService.CreateProject(ctx, "test-project", nil)
	require.NoError(t, err, "creating a clientless project for membership cleanup should succeed")
	otherProject, err := p.ProjectsService.CreateProject(ctx, "other-project", nil)
	require.NoError(t, err, "Creating the project that should remain assigned after deletion should succeed")
	queries := sqlc.New(p.PostgresPool)
	userIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for _, userID := range userIDs {
		_, err := queries.CreateUser(ctx, sqlc.CreateUserParams{
			ID: userID, Username: userID.String(), FirstName: "Test", LastName: "User", PasswordHash: "unused", Role: "user",
		})
		require.NoError(t, err, "create PostgreSQL user %s for membership cleanup", userID)
		require.NoError(t, queries.AddUserToProject(ctx, sqlc.AddUserToProjectParams{UserID: userID, ProjectID: project.ID}), "assign user %s to the project being deleted", userID)
	}
	require.NoError(t, queries.AddUserToProject(ctx, sqlc.AddUserToProjectParams{UserID: userIDs[2], ProjectID: otherProject.ID}), "assign third user to the project that should remain")

	require.NoError(t, p.ProjectsService.DeleteProject(ctx, project.ID.String()), "delete the clientless project with user memberships")
	_, err = p.ProjectsService.GetProject(ctx, project.ID.String())
	assert.EqualError(t, err, projects.ErrProjectNotFound, "project should not exist after deletion")

	for i, userID := range userIDs {
		_, err := queries.GetUserByID(ctx, userID)
		require.NoError(t, err, "user %s should survive project deletion", userID)
		memberships, err := queries.ListUserProjects(ctx, userID)
		require.NoError(t, err, "read user %s memberships after cascading deletion", userID)
		if i == 2 {
			require.Len(t, memberships, 1, "third user should retain only their other project membership")
			assert.Equal(t, otherProject.ID, memberships[0].ID, "deletion should preserve membership in the unrelated project")
		} else {
			assert.Empty(t, memberships, "user %s should have no memberships after their only project is deleted", userID)
		}
	}
}

func (p *ProjectsSuite) TestUpdateProject_DeactivatesAndReactivatesClients() {
	t := p.T()
	ctx := context.Background()
	project, err := p.ProjectsService.CreateProject(ctx, "client-lifecycle", []string{"client"})
	require.NoError(t, err, "create project with a client for lifecycle checks")
	require.Len(t, project.Clients, 1, "created project should expose one active client")
	client := project.Clients[0]
	assert.True(t, p.ProjectsService.ProjectExists(ctx, project.ID.String(), client.ID.String()), "active client UUID should be eligible for credentials and ingestion")
	assert.False(t, p.ProjectsService.ProjectExists(ctx, project.ID.String(), client.Name), "display names must not be accepted as client IDs")
	assert.True(t, p.ProjectsService.ProjectExists(ctx, project.ID.String(), "*"), "wildcard credentials should recognize an existing project")
	assert.False(t, p.ProjectsService.ProjectExists(ctx, uuid.NewString(), "*"), "wildcard credentials must not authorize a missing project")

	updated, err := p.ProjectsService.UpdateProject(ctx, project.ID, project.Name, nil)
	require.NoError(t, err, "removing a client from the form should deactivate it")
	assert.Empty(t, updated.Clients, "updated project should hide inactive clients")
	loaded, err := p.ProjectsService.GetProject(ctx, project.ID.String())
	require.NoError(t, err, "load project after deactivating its client")
	assert.Empty(t, loaded.Clients, "project reads should hide inactive clients")
	assert.False(t, p.ProjectsService.ProjectExists(ctx, project.ID.String(), client.ID.String()), "inactive client UUID must not authorize credentials or ingestion")
	belongs, err := p.ProjectsService.ClientBelongsToProject(ctx, project.ID.String(), client.ID.String())
	require.NoError(t, err, "check inactive client ownership for historical log access")
	assert.True(t, belongs, "inactive client should retain ownership for historical logs")
	stored, err := sqlc.New(p.PostgresPool).GetClientByID(ctx, client.ID)
	require.NoError(t, err, "deactivated client row must be retained")
	assert.False(t, stored.IsActive, "omitted client should be persisted as inactive rather than deleted")

	updated, err = p.ProjectsService.UpdateProject(ctx, project.ID, project.Name, []string{client.Name, client.Name})
	require.NoError(t, err, "restoring the same client name should reactivate its existing row")
	require.Len(t, updated.Clients, 1, "duplicate submitted names should yield one reactivated client")
	assert.Equal(t, client.ID, updated.Clients[0].ID, "reactivation must preserve the original client UUID")
	assert.True(t, p.ProjectsService.ProjectExists(ctx, project.ID.String(), client.ID.String()), "reactivated client should regain credentials and ingestion eligibility")
}

func (p *ProjectsSuite) TestDeleteProject_RejectsActiveAndInactiveClients() {
	t := p.T()
	ctx := context.Background()
	project, err := p.ProjectsService.CreateProject(ctx, "cannot-delete-clients", []string{"client"})
	require.NoError(t, err, "create project with a client for deletion restrictions")
	assert.EqualError(t, p.ProjectsService.DeleteProject(ctx, project.ID.String()), projects.ErrProjectHasClients, "project deletion must reject active clients")
	_, err = p.ProjectsService.UpdateProject(ctx, project.ID, project.Name, nil)
	require.NoError(t, err, "deactivate the client before retrying deletion")
	assert.EqualError(t, p.ProjectsService.DeleteProject(ctx, project.ID.String()), projects.ErrProjectHasClients, "project deletion must also reject retained inactive clients")
	_, err = p.ProjectsService.GetProject(ctx, project.ID.String())
	require.NoError(t, err, "rejected deletion must leave the project intact")
}

func (p *ProjectsSuite) TestSearchProjects() {
	t := p.Suite.T()
	ctx := context.Background()

	// Create test projects with various names
	testProjects := []struct {
		name    string
		clients []string
	}{
		{"frontend-app", []string{"client1"}},
		{"backend-api", []string{"client2"}},
		{"mobile-frontend", []string{"client3"}},
		{"data-pipeline", []string{"client4"}},
		{"analytics-dashboard", []string{"client5"}},
		{"user-service", []string{"client6"}},
		{"UPPERCASE-PROJECT", []string{"client7"}},
		{"special.chars-project", []string{"client8"}},
	}

	// Create all test projects
	for _, tp := range testProjects {
		_, err := p.ProjectsService.CreateProject(ctx, tp.name, tp.clients)
		assert.NoError(t, err, "Failed to create project: %s", tp.name)
	}

	testCases := []struct {
		name          string
		searchRequest projects.SearchProjectsRequest
		expectedNames []string
		expectedCount int
		wantErr       bool
		errorContains string
	}{
		{
			name: "search for 'frontend' - case insensitive",
			searchRequest: projects.SearchProjectsRequest{
				Search: "frontend",
				Page:   0,
				Size:   10,
			},
			expectedNames: []string{"frontend-app", "mobile-frontend"},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "search for 'api' - partial match",
			searchRequest: projects.SearchProjectsRequest{
				Search: "api",
				Page:   0,
				Size:   10,
			},
			expectedNames: []string{"backend-api"},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "search for 'service' - single result",
			searchRequest: projects.SearchProjectsRequest{
				Search: "service",
				Page:   0,
				Size:   10,
			},
			expectedNames: []string{"user-service"},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "search for 'project' - multiple results",
			searchRequest: projects.SearchProjectsRequest{
				Search: "project",
				Page:   0,
				Size:   10,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "search for 'uppercase' - case insensitive",
			searchRequest: projects.SearchProjectsRequest{
				Search: "uppercase",
				Page:   0,
				Size:   10,
			},
			expectedNames: []string{"UPPERCASE-PROJECT"},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "search for 'nonexistent' - no results",
			searchRequest: projects.SearchProjectsRequest{
				Search: "nonexistent",
				Page:   0,
				Size:   10,
			},
			expectedCount: 0,
			wantErr:       false,
		},
		{
			name: "pagination - page 0, size 2",
			searchRequest: projects.SearchProjectsRequest{
				Search: "",
				Page:   0,
				Size:   2,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "pagination - page 1, size 3",
			searchRequest: projects.SearchProjectsRequest{
				Search: "",
				Page:   1,
				Size:   3,
			},
			expectedCount: 3,
			wantErr:       false,
		},
		{
			name: "regex special chars escaped - dot",
			searchRequest: projects.SearchProjectsRequest{
				Search: "special.chars",
				Page:   0,
				Size:   10,
			},
			expectedNames: []string{"special.chars-project"},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "empty search - returns all within page size",
			searchRequest: projects.SearchProjectsRequest{
				Search: "",
				Page:   0,
				Size:   5,
			},
			expectedCount: 5,
			wantErr:       false,
		},
	}

	for i, tc := range testCases {
		results, err := p.ProjectsService.SearchProjects(ctx, tc.searchRequest)

		if tc.wantErr {
			assert.Error(t, err, "Test case %d (%s) should return error", i, tc.name)
			if tc.errorContains != "" {
				assert.Contains(t, err.Error(), tc.errorContains, "Test case %d (%s) error message", i, tc.name)
			}
			continue
		}

		assert.NoError(t, err, "Test case %d (%s) should not return error", i, tc.name)
		assert.Len(t, results, tc.expectedCount, "Test case %d (%s) expected %d results", i, tc.name, tc.expectedCount)

		// If specific names are expected, verify them
		if len(tc.expectedNames) > 0 {
			resultNames := make([]string, len(results))
			for j, project := range results {
				resultNames[j] = project.Name
			}

			for _, expectedName := range tc.expectedNames {
				assert.Contains(t, resultNames, expectedName, "Test case %d (%s) should contain %s", i, tc.name, expectedName)
			}
		}
	}
}

func (p *ProjectsSuite) TestGetProjectPage() {
	t := p.Suite.T()
	ctx := context.Background()

	// Create test projects
	testProjects := []struct {
		name    string
		clients []string
	}{
		{"project-1", []string{"client1"}},
		{"project-2", []string{"client2"}},
		{"project-3", []string{"client3"}},
		{"project-4", []string{"client4"}},
		{"project-5", []string{"client5"}},
		{"project-6", []string{"client6"}},
		{"project-7", []string{"client7"}},
		{"project-8", []string{"client8"}},
		{"project-9", []string{"client9"}},
		{"project-10", []string{"client10"}},
		{"project-11", []string{"client11"}},
		{"project-12", []string{"client12"}},
	}

	// Create all test projects
	for _, tp := range testProjects {
		_, err := p.ProjectsService.CreateProject(ctx, tp.name, tp.clients)
		assert.NoError(t, err, "Failed to create project: %s", tp.name)
	}

	testCases := []struct {
		name          string
		query         types.GetProjectPageInput
		expectedCount int
		expectedTotal int64
		expectedNames []string
		wantErr       bool
	}{
		{
			name: "first page with size 5",
			query: types.GetProjectPageInput{
				Page: 0,
				Size: 5,
			},
			expectedCount: 5,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "second page with size 5",
			query: types.GetProjectPageInput{
				Page: 1,
				Size: 5,
			},
			expectedCount: 5,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "third page with size 5",
			query: types.GetProjectPageInput{
				Page: 2,
				Size: 5,
			},
			expectedCount: 2,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "page beyond available data",
			query: types.GetProjectPageInput{
				Page: 5,
				Size: 5,
			},
			expectedCount: 0,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "page size 10",
			query: types.GetProjectPageInput{
				Page: 0,
				Size: 10,
			},
			expectedCount: 10,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "page 1 with size 10",
			query: types.GetProjectPageInput{
				Page: 1,
				Size: 10,
			},
			expectedCount: 2,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "all projects on single page",
			query: types.GetProjectPageInput{
				Page: 0,
				Size: 20,
			},
			expectedCount: 12,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "page size 1",
			query: types.GetProjectPageInput{
				Page: 0,
				Size: 1,
			},
			expectedCount: 1,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "search for 'project-1' - partial matches",
			query: types.GetProjectPageInput{
				Search: "project-1",
				Page:   0,
				Size:   10,
			},
			expectedCount: 4,
			expectedTotal: 4,
			expectedNames: []string{"project-1", "project-10", "project-11", "project-12"},
			wantErr:       false,
		},
		{
			name: "search for '-5' - single match",
			query: types.GetProjectPageInput{
				Search: "-5",
				Page:   0,
				Size:   10,
			},
			expectedCount: 1,
			expectedTotal: 1,
			expectedNames: []string{"project-5"},
			wantErr:       false,
		},
		{
			name: "search for '11' - partial match",
			query: types.GetProjectPageInput{
				Search: "11",
				Page:   0,
				Size:   10,
			},
			expectedCount: 1,
			expectedTotal: 1,
			expectedNames: []string{"project-11"},
			wantErr:       false,
		},
		{
			name: "search with pagination - page 0",
			query: types.GetProjectPageInput{
				Search: "project-",
				Page:   0,
				Size:   5,
			},
			expectedCount: 5,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "search with pagination - page 1",
			query: types.GetProjectPageInput{
				Search: "project-",
				Page:   1,
				Size:   5,
			},
			expectedCount: 5,
			expectedTotal: 12,
			wantErr:       false,
		},
		{
			name: "search with no results",
			query: types.GetProjectPageInput{
				Search: "nonexistent",
				Page:   0,
				Size:   10,
			},
			expectedCount: 0,
			expectedTotal: 0,
			wantErr:       false,
		},
		{
			name: "search case insensitive",
			query: types.GetProjectPageInput{
				Search: "PROJECT-5",
				Page:   0,
				Size:   10,
			},
			expectedCount: 1,
			expectedTotal: 1,
			expectedNames: []string{"project-5"},
			wantErr:       false,
		},
	}

	for i, tc := range testCases {
		results, totalCount, err := p.ProjectsService.GetProjectPage(ctx, tc.query)

		if tc.wantErr {
			assert.Error(t, err, "Test case %d (%s) should return error", i, tc.name)
			continue
		}

		assert.NoError(t, err, "Test case %d (%s) should not return error", i, tc.name)
		assert.Len(t, results, tc.expectedCount, "Test case %d (%s) expected %d results", i, tc.name, tc.expectedCount)
		assert.Equal(t, tc.expectedTotal, totalCount, "Test case %d (%s) expected total count %d", i, tc.name, tc.expectedTotal)

		// Verify projects have required fields (from aggregation)
		for _, project := range results {
			assert.NotEmpty(t, project.ID, "Project should have ID")
			assert.NotEmpty(t, project.Name, "Project should have name")
			assert.NotNil(t, project.Clients, "Project should have clients array")
		}

		// Verify projects are sorted by name
		for j := 1; j < len(results); j++ {
			assert.LessOrEqual(t, results[j-1].Name, results[j].Name,
				"Test case %d (%s) projects should be sorted by name", i, tc.name)
		}

		// If specific names are expected, verify them
		if len(tc.expectedNames) > 0 {
			resultNames := make([]string, len(results))
			for j, project := range results {
				resultNames[j] = project.Name
			}

			for _, expectedName := range tc.expectedNames {
				assert.Contains(t, resultNames, expectedName, "Test case %d (%s) should contain %s", i, tc.name, expectedName)
			}
		}
	}
}
