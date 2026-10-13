package auth_test

import (
	"context"
	"testing"

	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestPostgresAuthRejectsInvalidInputBeforeDatabaseAccess(t *testing.T) {
	service := auth.NewPostgresAuthService(nil, nil)
	_, err := service.CreateOrUpdateUser(context.Background(), types.CreateUserForm{ProjectIDs: []string{"mongo-project"}})
	require.Error(t, err, "Project assignment should be rejected before database access while it is temporarily disabled")
	err = service.ResetPassword(context.Background(), "unused", types.ResetPasswordForm{
		Password: "password1", RepeatedPassword: "password2",
	})
	require.EqualError(t, err, auth.PasswordsDoNotMatch, "Mismatched reset passwords should be rejected before accessing the database")
	_, _, err = service.GetUserPage(context.Background(), types.GetUserPageInput{Page: -1, Size: 10})
	require.Error(t, err, "A negative user page should be rejected before accessing the database")
}
