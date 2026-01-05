package auth

import (
	"context"

	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (suite *AuthSuite) TestAddUserToProject() {
	t := suite.T()
	ctx := context.Background()

	// Create a user
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		Role:      "user",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)

	// Create a project ID (simulating an existing project)
	projectID := primitive.NewObjectID()

	// Add user to project
	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), projectID.Hex())
	assert.NoError(t, err)

	// Verify user has the project
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 1)
	assert.Equal(t, projectID, updatedUser.ProjectIDs[0])
}

func (suite *AuthSuite) TestAddUserToProject_NoDuplicates() {
	t := suite.T()
	ctx := context.Background()

	// Create a user
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		Role:      "user",
	})
	assert.NoError(t, err)

	projectID := primitive.NewObjectID()

	// Add user to project twice
	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), projectID.Hex())
	assert.NoError(t, err)

	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), projectID.Hex())
	assert.NoError(t, err)

	// Verify no duplicates
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 1, "Should not have duplicate project IDs")
}

func (suite *AuthSuite) TestAddUserToProject_MultipleProjects() {
	t := suite.T()
	ctx := context.Background()

	// Create a user
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		Role:      "user",
	})
	assert.NoError(t, err)

	// Create multiple project IDs
	projectID1 := primitive.NewObjectID()
	projectID2 := primitive.NewObjectID()
	projectID3 := primitive.NewObjectID()

	// Add user to multiple projects
	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), projectID1.Hex())
	assert.NoError(t, err)

	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), projectID2.Hex())
	assert.NoError(t, err)

	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), projectID3.Hex())
	assert.NoError(t, err)

	// Verify user has all projects
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 3)
	assert.Contains(t, updatedUser.ProjectIDs, projectID1)
	assert.Contains(t, updatedUser.ProjectIDs, projectID2)
	assert.Contains(t, updatedUser.ProjectIDs, projectID3)
}

func (suite *AuthSuite) TestRemoveUserFromProject() {
	t := suite.T()
	ctx := context.Background()

	// Create a user with projects
	projectID1 := primitive.NewObjectID()
	projectID2 := primitive.NewObjectID()

	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID1.Hex(), projectID2.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, user.ProjectIDs, 2)

	// Remove user from one project
	err = suite.authService.RemoveUserFromProject(ctx, user.ID.Hex(), projectID1.Hex())
	assert.NoError(t, err)

	// Verify project was removed
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 1)
	assert.Equal(t, projectID2, updatedUser.ProjectIDs[0])
	assert.NotContains(t, updatedUser.ProjectIDs, projectID1)
}

func (suite *AuthSuite) TestRemoveUserFromProject_AllProjects() {
	t := suite.T()
	ctx := context.Background()

	// Create a user with one project
	projectID := primitive.NewObjectID()

	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, user.ProjectIDs, 1)

	// Remove user from the project
	err = suite.authService.RemoveUserFromProject(ctx, user.ID.Hex(), projectID.Hex())
	assert.NoError(t, err)

	// Verify user has no projects
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 0)
}

func (suite *AuthSuite) TestSetUserProjects() {
	t := suite.T()
	ctx := context.Background()

	// Create a user
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		Role:      "user",
	})
	assert.NoError(t, err)

	// Create project IDs
	projectID1 := primitive.NewObjectID()
	projectID2 := primitive.NewObjectID()
	projectID3 := primitive.NewObjectID()

	// Set user projects
	err = suite.authService.SetUserProjects(ctx, user.ID.Hex(), []string{
		projectID1.Hex(),
		projectID2.Hex(),
		projectID3.Hex(),
	})
	assert.NoError(t, err)

	// Verify projects were set
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 3)
	assert.Contains(t, updatedUser.ProjectIDs, projectID1)
	assert.Contains(t, updatedUser.ProjectIDs, projectID2)
	assert.Contains(t, updatedUser.ProjectIDs, projectID3)
}

func (suite *AuthSuite) TestSetUserProjects_ReplacesExisting() {
	t := suite.T()
	ctx := context.Background()

	// Create a user with initial projects
	oldProjectID := primitive.NewObjectID()

	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{oldProjectID.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, user.ProjectIDs, 1)

	// Set new projects (should replace old ones)
	newProjectID1 := primitive.NewObjectID()
	newProjectID2 := primitive.NewObjectID()

	err = suite.authService.SetUserProjects(ctx, user.ID.Hex(), []string{
		newProjectID1.Hex(),
		newProjectID2.Hex(),
	})
	assert.NoError(t, err)

	// Verify old projects were replaced
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 2)
	assert.Contains(t, updatedUser.ProjectIDs, newProjectID1)
	assert.Contains(t, updatedUser.ProjectIDs, newProjectID2)
	assert.NotContains(t, updatedUser.ProjectIDs, oldProjectID)
}

func (suite *AuthSuite) TestSetUserProjects_EmptyArray() {
	t := suite.T()
	ctx := context.Background()

	// Create a user with projects
	projectID := primitive.NewObjectID()

	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, user.ProjectIDs, 1)

	// Set empty projects array
	err = suite.authService.SetUserProjects(ctx, user.ID.Hex(), []string{})
	assert.NoError(t, err)

	// Verify all projects were removed
	updatedUser, err := suite.authService.GetUserByID(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 0)
}

func (suite *AuthSuite) TestUserHasAccessToProject_UserRole() {
	t := suite.T()
	ctx := context.Background()

	// Create a user with specific projects
	projectID1 := primitive.NewObjectID()
	projectID2 := primitive.NewObjectID()
	projectID3 := primitive.NewObjectID()

	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID1.Hex(), projectID2.Hex()},
	})
	assert.NoError(t, err)

	// Test access to projects user has
	hasAccess, err := suite.authService.UserHasAccessToProject(ctx, user.ID.Hex(), projectID1.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "User should have access to projectID1")

	hasAccess, err = suite.authService.UserHasAccessToProject(ctx, user.ID.Hex(), projectID2.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "User should have access to projectID2")

	// Test access to project user doesn't have
	hasAccess, err = suite.authService.UserHasAccessToProject(ctx, user.ID.Hex(), projectID3.Hex())
	assert.NoError(t, err)
	assert.False(t, hasAccess, "User should NOT have access to projectID3")
}

func (suite *AuthSuite) TestUserHasAccessToProject_AdminRole() {
	t := suite.T()
	ctx := context.Background()

	// Create an admin user with no specific projects
	adminUser, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "admin",
		FirstName: "Admin",
		LastName:  "User",
		Role:      "admin",
	})
	assert.NoError(t, err)

	// Admin should have access to any project
	projectID := primitive.NewObjectID()

	hasAccess, err := suite.authService.UserHasAccessToProject(ctx, adminUser.ID.Hex(), projectID.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "Admin should have access to all projects")
}

func (suite *AuthSuite) TestGetUsersForProject() {
	t := suite.T()
	ctx := context.Background()

	projectID := primitive.NewObjectID()

	// Create multiple users with access to the project
	user1, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "user1",
		FirstName:  "User",
		LastName:   "One",
		Role:       "user",
		ProjectIDs: []string{projectID.Hex()},
	})
	assert.NoError(t, err)

	user2, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "user2",
		FirstName:  "User",
		LastName:   "Two",
		Role:       "user",
		ProjectIDs: []string{projectID.Hex()},
	})
	assert.NoError(t, err)

	// Create a user without access to the project
	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "user3",
		FirstName: "User",
		LastName:  "Three",
		Role:      "user",
	})
	assert.NoError(t, err)

	// Get users for the project
	users, err := suite.authService.GetUsersForProject(ctx, projectID.Hex())
	assert.NoError(t, err)
	assert.Len(t, users, 2, "Should return only users with access to the project")

	// Verify the correct users were returned
	userIDs := []primitive.ObjectID{users[0].ID, users[1].ID}
	assert.Contains(t, userIDs, user1.ID)
	assert.Contains(t, userIDs, user2.ID)
}

func (suite *AuthSuite) TestGetUsersForProject_NoUsers() {
	t := suite.T()
	ctx := context.Background()

	projectID := primitive.NewObjectID()

	// Create users without access to the project
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "user1",
		FirstName: "User",
		LastName:  "One",
		Role:      "user",
	})
	assert.NoError(t, err)

	// Get users for the project
	users, err := suite.authService.GetUsersForProject(ctx, projectID.Hex())
	assert.NoError(t, err)
	assert.Len(t, users, 0, "Should return empty array when no users have access")
}

func (suite *AuthSuite) TestCreateOrUpdateUser_WithProjects() {
	t := suite.T()
	ctx := context.Background()

	projectID1 := primitive.NewObjectID()
	projectID2 := primitive.NewObjectID()

	// Create user with projects
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID1.Hex(), projectID2.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, user.ProjectIDs, 2)
	assert.Contains(t, user.ProjectIDs, projectID1)
	assert.Contains(t, user.ProjectIDs, projectID2)

	// Update user with different projects
	projectID3 := primitive.NewObjectID()

	updatedUser, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		ID:         user.ID.Hex(),
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID3.Hex()},
	})
	assert.NoError(t, err)
	assert.Len(t, updatedUser.ProjectIDs, 1)
	assert.Contains(t, updatedUser.ProjectIDs, projectID3)
	assert.NotContains(t, updatedUser.ProjectIDs, projectID1)
	assert.NotContains(t, updatedUser.ProjectIDs, projectID2)
}

func (suite *AuthSuite) TestGetProjectIDsForUser() {
	t := suite.T()
	ctx := context.Background()

	projectID1 := primitive.NewObjectID()
	projectID2 := primitive.NewObjectID()
	projectID3 := primitive.NewObjectID()

	// Create user with projects
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{projectID1.Hex(), projectID2.Hex(), projectID3.Hex()},
	})
	assert.NoError(t, err)

	// Get project IDs for user
	projectIDs, err := suite.authService.GetProjectIDsForUser(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, projectIDs, 3)
	assert.Contains(t, projectIDs, projectID1.Hex())
	assert.Contains(t, projectIDs, projectID2.Hex())
	assert.Contains(t, projectIDs, projectID3.Hex())
}

func (suite *AuthSuite) TestGetProjectIDsForUser_NoProjects() {
	t := suite.T()
	ctx := context.Background()

	// Create user without projects
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		Role:      "user",
	})
	assert.NoError(t, err)

	// Get project IDs for user
	projectIDs, err := suite.authService.GetProjectIDsForUser(ctx, user.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, projectIDs, 0)
}

func (suite *AuthSuite) TestUserProjectRelationship_Integration() {
	t := suite.T()
	ctx := context.Background()

	// Create a projects service for integration testing
	projectsCollection := suite.Collection("projects")
	projectsService := projects.NewProjectsService(projectsCollection, suite.userCollection, suite.MongoClient)
	defer projectsCollection.Drop(ctx)

	// Create actual projects
	project1, err := projectsService.CreateProject(ctx, "Project One", []string{"client1"})
	assert.NoError(t, err)

	project2, err := projectsService.CreateProject(ctx, "Project Two", []string{"client2"})
	assert.NoError(t, err)

	// Create user with access to project1
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:   "testuser",
		FirstName:  "Test",
		LastName:   "User",
		Role:       "user",
		ProjectIDs: []string{project1.ID.Hex()},
	})
	assert.NoError(t, err)

	// Verify user has access to project1 but not project2
	hasAccess, err := suite.authService.UserHasAccessToProject(ctx, user.ID.Hex(), project1.ID.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "User should have access to project1")

	hasAccess, err = suite.authService.UserHasAccessToProject(ctx, user.ID.Hex(), project2.ID.Hex())
	assert.NoError(t, err)
	assert.False(t, hasAccess, "User should NOT have access to project2")

	// Add user to project2
	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), project2.ID.Hex())
	assert.NoError(t, err)

	// Now user should have access to both
	hasAccess, err = suite.authService.UserHasAccessToProject(ctx, user.ID.Hex(), project2.ID.Hex())
	assert.NoError(t, err)
	assert.True(t, hasAccess, "User should now have access to project2")

	// Verify GetUsersForProject works
	usersForProject1, err := suite.authService.GetUsersForProject(ctx, project1.ID.Hex())
	assert.NoError(t, err)
	assert.Len(t, usersForProject1, 1)
	assert.Equal(t, user.ID, usersForProject1[0].ID)
}

func (suite *AuthSuite) TestInvalidObjectIDs() {
	t := suite.T()
	ctx := context.Background()

	// Test with invalid user ID
	err := suite.authService.AddUserToProject(ctx, "invalid", primitive.NewObjectID().Hex())
	assert.Error(t, err, "Should error with invalid user ID")

	// Test with invalid project ID
	user, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "testuser",
		FirstName: "Test",
		LastName:  "User",
		Role:      "user",
	})
	assert.NoError(t, err)

	err = suite.authService.AddUserToProject(ctx, user.ID.Hex(), "invalid")
	assert.Error(t, err, "Should error with invalid project ID")

	// Test UserHasAccessToProject with invalid IDs
	_, err = suite.authService.UserHasAccessToProject(ctx, "invalid", primitive.NewObjectID().Hex())
	assert.Error(t, err, "Should error with invalid user ID")

	// Test GetUsersForProject with invalid project ID
	_, err = suite.authService.GetUsersForProject(ctx, "invalid")
	assert.Error(t, err, "Should error with invalid project ID")
}
