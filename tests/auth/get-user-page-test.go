package auth

import (
	"context"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (suite *AuthSuite) TestGetUserPageNoSearch() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "bob",
		FirstName: "Bob",
		LastName:  "Jones",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "charlie",
		FirstName: "Charlie",
		LastName:  "Brown",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Get all users
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), totalCount)
	assert.Len(t, users, 3)
}

func (suite *AuthSuite) TestGetUserPageSearchUsername() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "bob",
		FirstName: "Bob",
		LastName:  "Jones",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "charlie",
		FirstName: "Charlie",
		LastName:  "Brown",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Search by username
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "alice",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), totalCount)
	assert.Len(t, users, 1)
	assert.Equal(t, "alice", users[0].Username)
}

func (suite *AuthSuite) TestGetUserPageSearchFirstName() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "bob",
		FirstName: "Bob",
		LastName:  "Jones",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "charlie",
		FirstName: "Charlie",
		LastName:  "Brown",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Search by first name
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "Bob",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), totalCount)
	assert.Len(t, users, 1)
	assert.Equal(t, "bob", users[0].Username)
	assert.Equal(t, "Bob", users[0].FirstName)
}

func (suite *AuthSuite) TestGetUserPageSearchLastName() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "bob",
		FirstName: "Bob",
		LastName:  "Jones",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "charlie",
		FirstName: "Charlie",
		LastName:  "Brown",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Search by last name
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "Brown",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), totalCount)
	assert.Len(t, users, 1)
	assert.Equal(t, "charlie", users[0].Username)
	assert.Equal(t, "Brown", users[0].LastName)
}

func (suite *AuthSuite) TestGetUserPageSearchCaseInsensitive() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "bob",
		FirstName: "Bob",
		LastName:  "Jones",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Search with different cases
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "ALICE",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), totalCount)
	assert.Len(t, users, 1)
	assert.Equal(t, "alice", users[0].Username)

	// Search with lowercase
	users, totalCount, err = suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "smith",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), totalCount)
	assert.Len(t, users, 1)
	assert.Equal(t, "Smith", users[0].LastName)
}

func (suite *AuthSuite) TestGetUserPagePagination() {
	t := suite.T()
	ctx := context.Background()

	// Create 5 test users
	for i := 1; i <= 5; i++ {
		_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
			Username:  "user" + string(rune('0'+i)),
			FirstName: "User",
			LastName:  "Test" + string(rune('0'+i)),
			Role:      "USER",
		})
		assert.NoError(t, err)
	}

	// Get first page (2 items)
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   2,
		Search: "",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(5), totalCount)
	assert.Len(t, users, 2)

	// Get second page (2 items)
	users, totalCount, err = suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   1,
		Size:   2,
		Search: "",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(5), totalCount)
	assert.Len(t, users, 2)

	// Get third page (1 item)
	users, totalCount, err = suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   2,
		Size:   2,
		Search: "",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(5), totalCount)
	assert.Len(t, users, 1)
}

func (suite *AuthSuite) TestGetUserPagePartialMatch() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "john_doe",
		FirstName: "John",
		LastName:  "Doe",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "jane_doe",
		FirstName: "Jane",
		LastName:  "Doe",
		Role:      "USER",
	})
	assert.NoError(t, err)

	_, err = suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "bob",
		FirstName: "Bob",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Search for partial match "doe" - should match both username and lastName
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "doe",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), totalCount)
	assert.Len(t, users, 2)
}

func (suite *AuthSuite) TestGetUserPageNoResults() {
	t := suite.T()
	ctx := context.Background()

	// Create test users
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Search for non-existent user
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "nonexistent",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(0), totalCount)
	assert.Len(t, users, 0)
}

func (suite *AuthSuite) TestGetUserPagePasswordNotReturned() {
	t := suite.T()
	ctx := context.Background()

	// Create test user
	_, err := suite.authService.CreateOrUpdateUser(ctx, types.CreateUserForm{
		Username:  "alice",
		FirstName: "Alice",
		LastName:  "Smith",
		Role:      "USER",
	})
	assert.NoError(t, err)

	// Get users
	users, totalCount, err := suite.authService.GetUserPage(ctx, types.GetUserPageInput{
		Page:   0,
		Size:   10,
		Search: "",
	})

	assert.NoError(t, err)
	assert.Equal(t, int64(1), totalCount)
	assert.Len(t, users, 1)

	// Password should not be returned (projection excludes it)
	// The password field should be empty string as it's excluded from the query
	assert.Empty(t, users[0].Password)
}
