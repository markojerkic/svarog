package postgres_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	store "github.com/markojerkic/svarog/internal/persistence/postgres"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAuthPersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("svarog_test"),
		postgres.WithUsername("svarog"),
		postgres.WithPassword("svarog"),
		postgres.WithInitScripts(filepath.Join("..", "..", "internal", "persistence", "postgres", "migrations", "000001_initial.up.sql")),
		postgres.BasicWaitStrategies(),
	)
	if container != nil {
		testcontainers.CleanupContainer(t, container)
	}
	require.NoError(t, err)
	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connectionString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	queries := sqlc.New(pool)

	run := func(name string, test func(t *testing.T)) {
		t.Run(name, func(t *testing.T) {
			_, err := pool.Exec(ctx, "TRUNCATE users CASCADE")
			require.NoError(t, err)
			test(t)
		})
	}
	create := func(t *testing.T, username string) sqlc.User {
		t.Helper()
		user, err := store.CreateUserWithToken(ctx, pool, authInput(username))
		require.NoError(t, err)
		return user
	}

	run("create and look up user", func(t *testing.T) {
		input := authInput("marko")
		user, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		require.Equal(t, input.ID, user.ID)
		require.Equal(t, input.PasswordHash, user.PasswordHash)
		require.Equal(t, input.FirstName, user.FirstName)
		require.Equal(t, input.LastName, user.LastName)
		require.Equal(t, input.Role, user.Role)
		require.Equal(t, input.NeedsPasswordReset, user.NeedsPasswordReset)
		require.False(t, user.CreatedAt.IsZero())
		byID, err := queries.GetUserByID(ctx, input.ID)
		require.NoError(t, err)
		require.Equal(t, user, byID)
		byUsername, err := queries.GetUserByUsername(ctx, input.Username)
		require.NoError(t, err)
		require.Equal(t, user, byUsername)
		_, err = store.CreateUserWithToken(ctx, pool, authInput("marko"))
		requireSQLState(t, err, "23505")
	})

	run("list and count literal search with stable pagination", func(t *testing.T) {
		for _, name := range []string{"Beta", "alpha", "ALPHA", "literal%name", "literal_name"} {
			create(t, name)
		}
		all, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 10})
		require.NoError(t, err)
		require.Len(t, all, 5)
		require.Equal(t, "Beta", all[2].Username)
		require.Equal(t, "literal%name", all[3].Username)
		require.Equal(t, "literal_name", all[4].Username)
		require.Less(t, all[0].ID.String(), all[1].ID.String())
		_, containsPassword := reflect.TypeOf(sqlc.ListUsersRow{}).FieldByName("PasswordHash")
		require.False(t, containsPassword)

		first, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 2})
		require.NoError(t, err)
		second, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 2, PageOffset: 2})
		require.NoError(t, err)
		third, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: "", PageSize: 2, PageOffset: 4})
		require.NoError(t, err)
		require.Equal(t, all, append(append(first, second...), third...))

		for search, expected := range map[string]int{"": 5, "ALpHa": 2, "%": 1, "_": 1, "First": 5, "Last": 5, "missing": 0} {
			rows, err := queries.ListUsers(ctx, sqlc.ListUsersParams{Search: search, PageSize: 10})
			require.NoError(t, err)
			count, err := queries.CountUsers(ctx, search)
			require.NoError(t, err)
			require.Len(t, rows, expected, search)
			require.Equal(t, int64(expected), count, search)
		}
	})

	run("profile and password update only intended fields", func(t *testing.T) {
		input := authInput("marko")
		user, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		updated, err := queries.UpdateUserProfile(ctx, sqlc.UpdateUserProfileParams{
			ID: user.ID, Username: "renamed", FirstName: "New", LastName: "Name", Role: "admin",
		})
		require.NoError(t, err)
		require.Equal(t, "renamed", updated.Username)
		require.Equal(t, "New", updated.FirstName)
		require.Equal(t, "Name", updated.LastName)
		require.Equal(t, "admin", updated.Role)
		require.Equal(t, user.PasswordHash, updated.PasswordHash)
		require.Equal(t, user.NeedsPasswordReset, updated.NeedsPasswordReset)
		require.Equal(t, user.CreatedAt, updated.CreatedAt)
		require.False(t, updated.UpdatedAt.Before(user.UpdatedAt))

		reset, err := queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
			ID: user.ID, PasswordHash: "new-hash", NeedsPasswordReset: false,
		})
		require.NoError(t, err)
		require.Equal(t, "new-hash", reset.PasswordHash)
		require.False(t, reset.NeedsPasswordReset)
		require.Equal(t, updated.Username, reset.Username)
		require.Equal(t, updated.Role, reset.Role)
		owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.NoError(t, err)
		require.Equal(t, user.ID, owner)
	})

	run("invalid token rolls back user creation", func(t *testing.T) {
		input := authInput("marko")
		input.Token.Hash = []byte("invalid")
		user, err := store.CreateUserWithToken(ctx, pool, input)
		requireSQLState(t, err, "23514")
		require.Equal(t, sqlc.User{}, user)
		_, err = queries.GetUserByID(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		count, err := queries.CountUsers(ctx, "")
		require.NoError(t, err)
		require.Zero(t, count)
	})

	run("replace token and password atomically", func(t *testing.T) {
		input := authInput("marko")
		input.NeedsPasswordReset = false
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		newToken := authInput("replacement").Token
		err = store.ReplaceLoginToken(ctx, pool, store.ReplaceLoginTokenInput{
			UserID: input.ID, PasswordHash: "replacement-hash", Token: newToken,
		})
		require.NoError(t, err)
		user, err := queries.GetUserByID(ctx, input.ID)
		require.NoError(t, err)
		require.Equal(t, "replacement-hash", user.PasswordHash)
		require.True(t, user.NeedsPasswordReset)
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		owner, err := queries.ConsumeLoginToken(ctx, newToken.Hash)
		require.NoError(t, err)
		require.Equal(t, input.ID, owner)
	})

	run("invalid replacement preserves password and token", func(t *testing.T) {
		input := authInput("marko")
		input.NeedsPasswordReset = false
		before, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		err = store.ReplaceLoginToken(ctx, pool, store.ReplaceLoginTokenInput{
			UserID: input.ID, PasswordHash: "replacement-hash",
			Token: store.LoginTokenInput{Hash: []byte("invalid"), ExpiresAt: time.Now().Add(time.Hour)},
		})
		requireSQLState(t, err, "23514")
		after, err := queries.GetUserByID(ctx, input.ID)
		require.NoError(t, err)
		require.Equal(t, before, after)
		owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.NoError(t, err)
		require.Equal(t, input.ID, owner)
	})

	run("token is consumed only once", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		owner, err := queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.NoError(t, err)
		require.Equal(t, input.ID, owner)
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows)
	})

	run("concurrent consume has one winner", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
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
				require.ErrorIs(t, err, pgx.ErrNoRows)
			}
		}
		require.Equal(t, 1, wins)
	})

	run("expired token cannot be consumed", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE login_tokens
			SET created_at = now() - interval '2 hours', expires_at = now() - interval '1 hour'
			WHERE user_id = $1`, input.ID)
		require.NoError(t, err)
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows)
	})

	run("delete user cascades to token", func(t *testing.T) {
		input := authInput("marko")
		_, err := store.CreateUserWithToken(ctx, pool, input)
		require.NoError(t, err)
		deleted, err := queries.DeleteUser(ctx, input.ID)
		require.NoError(t, err)
		require.Equal(t, input.ID, deleted)
		_, err = queries.GetUserByID(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		_, err = queries.ConsumeLoginToken(ctx, input.Token.Hash)
		require.ErrorIs(t, err, pgx.ErrNoRows)
		_, err = queries.DeleteUser(ctx, input.ID)
		require.ErrorIs(t, err, pgx.ErrNoRows)
	})

	run("missing user cannot get replacement token", func(t *testing.T) {
		input := authInput("missing")
		err := store.ReplaceLoginToken(ctx, pool, store.ReplaceLoginTokenInput{
			UserID: input.ID, PasswordHash: input.PasswordHash, Token: input.Token,
		})
		require.ErrorIs(t, err, pgx.ErrNoRows)
		err = queries.UpsertLoginToken(ctx, sqlc.UpsertLoginTokenParams{
			UserID: input.ID, TokenHash: input.Token.Hash, ExpiresAt: input.Token.ExpiresAt,
		})
		requireSQLState(t, err, "23503")
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

func requireSQLState(t *testing.T, err error, code string) {
	t.Helper()
	var pgError *pgconn.PgError
	require.ErrorAs(t, err, &pgError)
	require.Equal(t, code, pgError.Code)
}
