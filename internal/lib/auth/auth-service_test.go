package auth_test

import (
	"context"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestPostgresAuthRejectsInvalidInputBeforeDatabaseAccess(t *testing.T) {
	service := auth.NewPostgresAuthService(nil, nil)

	_, err := service.CreateOrUpdateUser(context.Background(), types.CreateUserForm{
		ProjectIDs: []string{uuid.NewString(), "not-a-uuid"},
	})

	var apiError types.ApiError
	require.ErrorAs(t, err, &apiError, "Invalid project UUID should be rejected before database access")
	require.NotEmpty(t, apiError.Fields["projects"], "Invalid project UUID should identify the projects validation field")

	_, err = service.CreateOrUpdateUser(context.Background(), types.CreateUserForm{
		ID:         uuid.NewString(),
		ProjectIDs: []string{uuid.NewString(), "not-a-uuid"},
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

func TestUserProfileValidationBeforeDatabaseAccess(t *testing.T) {
	service := auth.NewPostgresAuthService(nil, nil)
	ctx := i18n.WithLanguage(context.Background(), i18n.Croatian)
	for _, tc := range []struct {
		name  string
		field string
		value string
	}{
		{"blank username", "username", "   "},
		{"blank first name", "firstName", "\u00a0\u2003 "},
		{"blank last name", "lastName", "   "},
		{"short trimmed name", "firstName", " x "},
		{"long username", "username", strings.Repeat("Ž", 256)},
		{"long first name", "firstName", strings.Repeat("Ž", 256)},
		{"long last name", "lastName", strings.Repeat("Ž", 256)},
		{"NUL username", "username", "user\x00name"},
		{"NUL first name", "firstName", "Ime\x00"},
		{"NUL last name", "lastName", "Prezime\x00"},
		{"invalid UTF8", "firstName", "Ime\xff"},
		{"unknown role", "role", "owner"},
		{"missing role", "role", ""},
		{"malformed user ID", "id", "not-a-uuid"},
		{"nil user ID", "id", uuid.Nil.String()},
		{"nil project ID", "projects", uuid.Nil.String()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := types.CreateUserForm{Username: "username", FirstName: "Željko", LastName: "Čačić", Role: "user"}
			switch tc.field {
			case "username":
				form.Username = tc.value
			case "firstName":
				form.FirstName = tc.value
			case "lastName":
				form.LastName = tc.value
			case "role":
				form.Role = tc.value
			case "id":
				form.ID = tc.value
			case "projects":
				form.ProjectIDs = []string{tc.value}
			}
			_, err := service.CreateOrUpdateUser(ctx, form)
			var apiErr types.ApiError
			require.ErrorAs(t, err, &apiErr, "Invalid user input must fail as a typed client error before hashing or database access")
			require.NotEmpty(t, apiErr.Fields[tc.field], "Validation must identify the rejected user field")
			require.Equal(t, "Podaci nisu ispravni.", apiErr.Message, "Validation errors must use the request language")
		})
	}
	require.NoError(t, auth.ValidateUserForm(ctx, types.CreateUserForm{
		Username: strings.Repeat("Ž", 255), FirstName: strings.Repeat("ć", 255), LastName: "Čačić 👩‍💻", Role: "admin",
	}), "Names at the Unicode character boundary must remain accepted")
}

func TestUserSearchAndPaginationValidation(t *testing.T) {
	service := auth.NewPostgresAuthService(nil, nil)
	for _, query := range []types.GetUserPageInput{
		{Page: -1, Size: 10}, {Page: math.MaxInt32, Size: 10}, {Page: math.MaxInt64, Size: 10},
		{Size: 0}, {Size: 101}, {Size: 10, Search: "\x00"}, {Size: 10, Search: strings.Repeat("Ž", 1025)},
	} {
		_, _, err := service.GetUserPage(context.Background(), query)
		var apiErr types.ApiError
		require.ErrorAs(t, err, &apiErr, "Invalid paging and search must fail as client errors before PostgreSQL access")
	}
	require.NoError(t, auth.ValidateUserPage(context.Background(), types.GetUserPageInput{Page: math.MaxInt32 / 100, Size: 100}), "The largest safe int32 offset must remain valid")
	require.Error(t, auth.ValidateUserPage(context.Background(), types.GetUserPageInput{Page: math.MaxInt32/100 + 1, Size: 100}), "An offset beyond int32 must be rejected before narrowing")
}

func FuzzUserProfileTextValidation(f *testing.F) {
	for _, value := range []string{"Željko Čačić", "   ", "👩‍💻", "name\x00", "name\xff", strings.Repeat("Ž", 256)} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		err := auth.ValidateUserForm(context.Background(), types.CreateUserForm{
			Username: "fuzz-user", FirstName: value, LastName: value, Role: "user",
		})
		if strings.ContainsRune(value, '\x00') || !utf8.ValidString(value) || utf8.RuneCountInString(strings.TrimSpace(value)) < 3 || utf8.RuneCountInString(value) > 255 {
			var apiErr types.ApiError
			require.ErrorAs(t, err, &apiErr, "Invalid profile text must always produce a typed validation error")
			require.NotEmpty(t, apiErr.Fields["firstName"], "Invalid names must never reach PostgreSQL text fields")
		}
	})
}
