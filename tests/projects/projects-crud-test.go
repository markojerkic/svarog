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
