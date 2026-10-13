package postgres_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/markojerkic/svarog/tests/testutils"
	"github.com/stretchr/testify/require"
)

func TestAuthPersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pool := testutils.NewPostgresPool(t, ctx)
	queries := sqlc.New(pool)

	run := func(name string, test func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE users CASCADE")
			require.NoError(t, err, "clear users and dependent rows before %s", name)
			test(t)
		})
	}
	create := func(t *testing.T, username string) sqlc.User {
		t.Helper()
		user, err := store.CreateUserWithToken(ctx, pool, authInput(username))
		require.NoError(t, err, "create fixture user %q with login token", username)
		return user
	}

	run("create and look up user", func(t *testing.T) {
		input := authInput("marko")
		user, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user and token atomically")
		require.Equal(t, input.ID, user.ID, "persist supplied user ID")
		require.Equal(t, input.PasswordHash, user.PasswordHash, "persist password hash")
		require.Equal(t, input.FirstName, user.FirstName, "persist first name")
		require.Equal(t, input.LastName, user.LastName, "persist last name")
		require.Equal(t, input.Role, user.Role, "persist role")
		require.Equal(t, input.NeedsPasswordReset, user.NeedsPasswordReset, "persist password reset requirement")
		require.False(t, user.CreatedAt.IsZero(), "set creation timestamp")
		byID, err := queries.GetUserByID(ctx, input.ID)
		require.NoError(t, err, "look up created user by ID")
		require.Equal(t, user, byID, "ID lookup returns the persisted user")
		byUsername, err := queries.GetUserByUsername(ctx, input.Username)
		require.NoError(t, err, "look up created user by username")
		require.Equal(t, user, byUsername, "username lookup returns the persisted user")
		_, err = store.CreateUserWithToken(ctx, pool, authInput("marko"))
		requireSQLState(t, err, "23505", "reject duplicate username")
	})

	run("list and count literal search with stable pagination", func(t *testing.T) {
		for _, name := range []string{"Beta", "alpha", "ALPHA", "literal%name", "literal_name"} {
			create(t, name)
		}
		all, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 10})
		require.NoError(t, err, "list all users without a search filter")
		require.Len(t, all, 5, "return every fixture user")
		require.Equal(t, "Beta", all[2].Username, "sort usernames case-insensitively")
		require.Equal(t, "literal%name", all[3].Username, "sort percent-containing username after Beta")
		require.Equal(t, "literal_name", all[4].Username, "sort underscore-containing username last")
		require.Less(t, all[0].ID.String(), all[1].ID.String(), "break equal case-insensitive usernames by ID")
		_, containsPassword := reflect.TypeOf(sqlc.ListUsersRow{}).FieldByName("PasswordHash")
		require.False(t, containsPassword, "list rows must not expose password hashes")

		first, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 2})
		require.NoError(t, err, "fetch first user page")
		second, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 2, PageOffset: 2})
		require.NoError(t, err, "fetch second user page")
		third, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 2, PageOffset: 4})
		require.NoError(t, err, "fetch third user page")
		require.Equal(t, all, append(append(first, second...), third...), "pagination preserves ordering without omissions or duplicates")

		for search, expected := range map[string]int{"": 5, "ALpHa": 2, "%": 1, "_": 1, "First": 5, "Last": 5, "missing": 0} {
			rows, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: search, PageSize: 10})
			require.NoError(t, err, "list users matching literal case-insensitive search %q", search)
			count, err := queries.CountUsers(ctx, search)
			require.NoError(t, err, "count users matching search %q", search)
			require.Len(t, rows, expected, "literal case-insensitive search %q returns expected users", search)
			require.Equal(t, int64(expected), count, "count agrees with list for search %q", search)
		}
	})

	run("profile and password update only intended fields", func(t *testing.T) {
		input := authInput("marko")
		user, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user before profile update")
		updated, err := queries.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
			ID: user.ID, Username: "renamed", FirstName: "New", LastName: "Name", Role: "admin",
		})
		require.NoError(t, err, "update user profile")
		require.Equal(t, "renamed", updated.Username, "update username")
		require.Equal(t, "New", updated.FirstName, "update first name")
		require.Equal(t, "Name", updated.LastName, "update last name")
		require.Equal(t, "admin", updated.Role, "update role")
		require.Equal(t, user.PasswordHash, updated.PasswordHash, "profile update preserves password hash")
		require.Equal(t, user.NeedsPasswordReset, updated.NeedsPasswordReset, "profile update preserves reset requirement")
		require.Equal(t, user.CreatedAt, updated.CreatedAt, "profile update preserves creation timestamp")
		require.False(t, updated.UpdatedAt.Before(user.UpdatedAt), "profile update must not move update timestamp backwards")

		reset, err := queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
			ID: user.ID, PasswordHash: "new-hash", NeedsPasswordReset: false,
		})
		require.NoError(t, err, "update password and reset requirement")
		require.Equal(t, "new-hash", reset.PasswordHash, "persist replacement password hash")
		require.False(t, reset.NeedsPasswordReset, "clear reset requirement")
		require.Equal(t, updated.Username, reset.Username, "password update preserves username")
		require.Equal(t, updated.Role, reset.Role, "password update preserves role")
		owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.NoError(t, err, "consume token after profile and password updates")
		require.Equal(t, user.ID, owner, "updates preserve token ownership")
	})

	run("invalid token rolls back user creation", func(t *testing.T) {
		input := authInput("marko")
		input.Token.Hash = []byte("invalid")
		user, err := store.CreateUserWithToken(ctx, pool, input)
		requireSQLState(t, err, "23514", "reject invalid token hash during user creation")
		require.Equal(t, sqlc.User{}, user, "failed transaction returns no user")
		_, err = queries.GetUserByID(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows, "invalid token rolls back inserted user")
		count, err := queries.CountUsers(ctx, "")
		require.NoError(t, err, "count users after failed creation")
		require.Zero(t, count, "failed creation leaves no users")
	})

	run("replace token and password atomically", func(t *testing.T) {
		input := authInput("marko")
		input.NeedsPasswordReset = false
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user before token replacement")
		newToken := authInput("replacement").Token
		err = store.ReplaceLoginToken(ctx, pool, store.ReplaceLoginTokenInput{
			UserID: input.ID, PasswordHash: "replacement-hash", Token: newToken,
		})
		require.NoError(t, err, "replace token and password atomically")
		user, err := queries.GetUserByID(ctx, input.ID)
		require.NoError(t, err, "reload user after token replacement")
		require.Equal(t, "replacement-hash", user.PasswordHash, "replacement updates password hash")
		require.True(t, user.NeedsPasswordReset, "replacement requires password reset")
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows, "replacement invalidates previous token")
		owner, err := queries.ConsumeLoginToken(ctx, newToken.Hash)
		require.NoError(t, err, "consume replacement token")
		require.Equal(t, input.ID, owner, "replacement token belongs to original user")
	})

	run("invalid replacement preserves password and token", func(t *testing.T) {
		input := authInput("marko")
		input.NeedsPasswordReset = false
		before, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user before invalid replacement")
		err = store.ReplaceLoginToken(ctx, pool, store.ReplaceLoginTokenInput{
			UserID: input.ID, PasswordHash: "replacement-hash",
			Token: store.LoginTokenInput{Hash: []byte("invalid"), ExpiresAt: time.Now().Add(time.Hour)},
		})
		requireSQLState(t, err, "23514", "reject invalid replacement token hash")
		after, err := queries.GetUserByID(ctx, input.ID)
		require.NoError(t, err, "reload user after failed replacement")
		require.Equal(t, before, after, "failed replacement preserves all user fields")
		owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.NoError(t, err, "original token remains consumable after failed replacement")
		require.Equal(t, input.ID, owner, "failed replacement preserves original token owner")
	})

	run("token is consumed only once", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user for single-use token test")
		owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.NoError(t, err, "first token consumption succeeds")
		require.Equal(t, input.ID, owner, "consumed token identifies its owner")
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows, "second token consumption is rejected")
	})

	run("concurrent consume has one winner", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user for concurrent token consumption")
		start := make(chan struct{})
		results := make(chan error, 2)
		for range 2 {
			go func() {
				<-start
				owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
				if err == nil && owner != input.ID {
					err = errors.New("token returned wrong user")
				}
				results <- err
			}()
		}
		close(start)
		wins := 0
		for range 2 {
			err := <-results
			if err == nil {
				wins++
			} else {
				require.ErrorIs(t, err, pgx.ErrNoRows, "losing concurrent consumer sees no available token")
			}
		}
		require.Equal(t, 1, wins, "exactly one concurrent token consumer succeeds")
	})

	run("expired token cannot be consumed", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user for expired token test")
		_, err = pool.Exec(ctx, `UPDATE login_tokens
			SET created_at = now() - interval '2 hours', expires_at = now() - interval '1 hour'
			WHERE user_id = $1`, input.ID)
		require.NoError(t, err, "expire fixture login token")
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows, "expired token cannot be consumed")
	})

	run("delete user cascades to token", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err, "create user before cascade deletion")
		deleted, err := queries.DeleteUser(ctx, input.ID)
		require.NoError(t, err, "delete user with token")
		require.Equal(t, input.ID, deleted, "delete returns removed user ID")
		_, err = queries.GetUserByID(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows, "deleted user is no longer retrievable")
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows, "user deletion cascades to login token")
		_, err = queries.DeleteUser(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows, "deleting already removed user returns no rows")
	})

	run("missing user cannot get replacement token", func(t *testing.T) {
		input := authInput("missing")
		err := store.ReplaceLoginToken(ctx, pool, store.ReplaceLoginTokenInput{
			UserID: input.ID, PasswordHash: input.PasswordHash, Token: input.Token,
		})
		require.ErrorIs(t, err, pgx.ErrNoRows, "cannot replace token for missing user")
		err = queries.UpsertLoginToken(ctx, sqlc.UpsertLoginTokenParams{
			UserID: input.ID, TokenHash: input.Token.Hash, ExpiresAt: input.Token.ExpiresAt,
		})
		requireSQLState(t, err, "23503", "token foreign key rejects missing parent user")
	})
}

func authInput(username string) store.CreateUserWithTokenInput {
	hash := sha256.Sum256([]byte(username))
	return store.CreateUserWithTokenInput{
		ID: uuid.New(), Username: username, FirstName: "First", LastName: "Last",
		PasswordHash: "fixture-password-hash", Role: "user", NeedsPasswordReset: true,
		Token: store.LoginTokenInput{Hash: hash[:], ExpiresAt: time.Now().Add(time.Hour)},
	}
}

func requireSQLState(t *testing.T, err error, code string, msgAndArgs ...any) {
	t.Helper()
	var pgError *pgconn.PgError
	require.ErrorAs(t, err, &pgError, msgAndArgs...)
	require.Equal(t, code, pgError.Code, msgAndArgs...)
}
