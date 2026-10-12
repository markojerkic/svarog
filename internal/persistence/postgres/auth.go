package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

func CreateUserWithToken(ctx context.Context, pool *pgxpool.Pool, input CreateUserWithTokenInput) (sqlc.User, error) {
	var user sqlc.User
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		queries := sqlc.New(pool).WithTx(tx)
		var err error
		user, err = queries.CreateUser(ctx, sqlc.CreateUserParams{
			ID:                 input.ID,
			Username:           input.Username,
			FirstName:          input.FirstName,
			LastName:           input.LastName,
			PasswordHash:       input.PasswordHash,
			Role:               input.Role,
			NeedsPasswordReset: input.NeedsPasswordReset,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if err := queries.UpsertLoginToken(ctx, sqlc.UpsertLoginTokenParams{
			TokenHash: input.Token.Hash,
			UserID:    user.ID,
			ExpiresAt: input.Token.ExpiresAt,
		}); err != nil {
			return fmt.Errorf("create login token: %w", err)
		}
		return nil
	})
	if err != nil {
		return sqlc.User{}, fmt.Errorf("create user with token: %w", err)
	}
	return user, nil
}

func ReplaceLoginToken(ctx context.Context, pool *pgxpool.Pool, input ReplaceLoginTokenInput) error {
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		queries := sqlc.New(pool).WithTx(tx)
		if _, err := queries.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
			ID:                 input.UserID,
			PasswordHash:       input.PasswordHash,
			NeedsPasswordReset: true,
		}); err != nil {
			return fmt.Errorf("update user password: %w", err)
		}
		if err := queries.UpsertLoginToken(ctx, sqlc.UpsertLoginTokenParams{
			TokenHash: input.Token.Hash,
			UserID:    input.UserID,
			ExpiresAt: input.Token.ExpiresAt,
		}); err != nil {
			return fmt.Errorf("replace login token: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("replace user login token: %w", err)
	}
	return nil
}
