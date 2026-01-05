package projects

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		_, err := p.ProjectsService.CreateProject(context.Background(), tc.name, tc.clients)
		if tc.wantErr {
			assert.Error(t, err, "Test case %d failed", i)
		} else {
			assert.NoError(t, err, "Test case %d failed", i)
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
			id:      project.ID.Hex(),
			clients: []string{"test client"},
			wantErr: false,
		},
		{
			name:    "test2",
			id:      primitive.NewObjectID().Hex(),
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

	project, err := p.ProjectsService.CreateProject(context.Background(), "test1", []string{"test client"})
	if err != nil {
		t.Fatalf("Could not create project: %s", err)
	}

	err = p.ProjectsService.DeleteProject(context.Background(), project.ID.Hex())
	assert.NoError(t, err)

	_, err = p.ProjectsService.GetProject(context.Background(), project.ID.Hex())
	assert.Error(t, err)
}

func (p *ProjectsSuite) TestDeleteProject_CleansUpUserReferences() {
	t := p.Suite.T()
	ctx := context.Background()

	// Create a project
	project, err := p.ProjectsService.CreateProject(ctx, "test-project", []string{"test-client"})
	assert.NoError(t, err)
	assert.NotEmpty(t, project.ID)

	// Create multiple users with access to this project
	user1, err := p.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "user1",
		FirstName:  "User",
		LastName:   "One",
		Role:       "user",
		ProjectIDs: []string{project.ID.Hex()},
	})
	assert.NoError(t, err)

	user2, err := p.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "user2",
		FirstName:  "User",
		LastName:   "Two",
		Role:       "user",
		ProjectIDs: []string{project.ID.Hex()},
	})
	assert.NoError(t, err)

	// Create a user with multiple projects (including the one to be deleted)
	otherProject, err := p.ProjectsService.CreateProject(ctx, "other-project", []string{"other-client"})
	assert.NoError(t, err)

	user3, err := p.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "user3",
		FirstName:  "User",
		LastName:   "Three",
		Role:       "user",
		ProjectIDs: []string{project.ID.Hex(), otherProject.ID.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, user3.ProjectIDs, 2, "User3 should have 2 projects initially")

	// Verify users have access to the project
	hasAccess, err := p.authService.UserHasAccessToProject(ctx, user1.ID.Hex(), project.ID.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "User1 should have access before deletion")

	hasAccess, err = p.authService.UserHasAccessToProject(ctx, user2.ID.Hex(), project.ID.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "User2 should have access before deletion")

	// Delete the project
	err = p.ProjectsService.DeleteProject(ctx, project.ID.Hex())
	assert.NoError(t, err)

	// Verify project is deleted
	_, err = p.ProjectsService.GetProject(ctx, project.ID.Hex())
	assert.Error(t, err, "Project should not exist after deletion")

	// Verify users no longer have the project in their project_ids
	updatedUser1, err := p.authService.GetUserByID(ctx, user1.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser1.ProjectIDs, 0, "User1 should have no projects after deletion")
	assert.NotContains(t, updatedUser1.ProjectIDs, project.ID, "User1 should not have deleted project")

	updatedUser2, err := p.authService.GetUserByID(ctx, user2.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser2.ProjectIDs, 0, "User2 should have no projects after deletion")
	assert.NotContains(t, updatedUser2.ProjectIDs, project.ID, "User2 should not have deleted project")

	updatedUser3, err := p.authService.GetUserByID(ctx, user3.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser3.ProjectIDs, 1, "User3 should have 1 project remaining")
	assert.NotContains(t, updatedUser3.ProjectIDs, project.ID, "User3 should not have deleted project")
	assert.Contains(t, updatedUser3.ProjectIDs, otherProject.ID, "User3 should still have other project")

	// Verify users no longer have access via access check
	hasAccess, err = p.authService.UserHasAccessToProject(ctx, user1.ID.Hex(), project.ID.Hex())
	assert.NoError(t, err)
	assert.False(t, hasAccess, "User1 should not have access after deletion")

	hasAccess, err = p.authService.UserHasAccessToProject(ctx, user2.ID.Hex(), project.ID.Hex())
	assert.NoError(t, err)
	assert.False(t, hasAccess, "User2 should not have access after deletion")
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
