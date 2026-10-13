package auth_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestPostgresAuthRejectsInvalidInputBeforeDatabaseAccess(t *testing.T) {
	service := auth.NewPostgresAuthService(nil, nil)

	_, err := service.CreateOrUpdateUser(context.Background(), types.CreateUserForm{
		ProjectIDs: []string{uuid.NewString(), "mongo-project"},
	})

	var apiError types.ApiError
	require.ErrorAs(t, err, &apiError, "Invalid project UUID should be rejected before database access")
	require.NotEmpty(t, apiError.Fields["projects"], "Invalid project UUID should identify the projects validation field")

	_, err = service.CreateOrUpdateUser(context.Background(), types.CreateUserForm{
		ID:         uuid.NewString(),
		ProjectIDs: []string{uuid.NewString(), "mongo-project"},
	})

	require.ErrorAs(t, err, &apiError, "Invalid project UUID should be rejected before database access on profile updates")

	access, err := service.UserHasAccessToProject(context.Background(), "invalid-user", "invalid-project")

	require.NoError(t, err, "Malformed access identifiers should be denied without querying the database")
	require.False(t, access, "Malformed identifiers must never grant project access")

	access, err = service.UserHasAccessToProject(context.Background(), uuid.NewString(), "invalid-project")

	require.NoError(t, err, "Malformed project identifier should be denied before database access for a valid user UUID")
	require.False(t, access, "Malformed project identifier must never grant project access")

	err = service.ResetPassword(context.Background(), "unused", types.ResetPasswordForm{
		Password:         "password1",
		RepeatedPassword: "password2",
	})

	require.EqualError(t, err, auth.PasswordsDoNotMatch, "Mismatched reset passwords should be rejected before accessing the database")

	_, _, err = service.GetUserPage(context.Background(), types.GetUserPageInput{
		Page: -1,
		Size: 10,
	})

	require.Error(t, err, "A negative user page should be rejected before accessing the database")
}
