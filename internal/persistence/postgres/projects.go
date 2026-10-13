package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/markojerkic/svarog/internal/persistence/postgres/sqlc"
)

func CreateProjectWithClients(ctx context.Context, pool *pgxpool.Pool, input ProjectWithClientsInput) (sqlc.Project, error) {
	var project sqlc.Project
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var err error
		project, err = q.CreateProject(ctx, sqlc.CreateProjectParams{ID: input.ID, Name: input.Name})
		if err != nil {
			return err
		}
		for _, name := range input.Clients {
			if _, err := q.ActivateClientByName(ctx, sqlc.ActivateClientByNameParams{
				ID: uuid.New(), ProjectID: input.ID, Name: name,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return sqlc.Project{}, fmt.Errorf("create project with clients: %w", err)
	}
	return project, nil
}

func UpdateProjectWithClients(ctx context.Context, pool *pgxpool.Pool, input ProjectWithClientsInput) (sqlc.Project, error) {
	var project sqlc.Project
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := sqlc.New(tx)
		var err error

		project, err = q.UpdateProject(ctx, sqlc.UpdateProjectParams{ID: input.ID, Name: input.Name})
		if err != nil {
			return err
		}
		names := input.Clients
		if names == nil {
			names = []string{}
		}
		if err := q.DeactivateUnlistedClients(ctx, sqlc.DeactivateUnlistedClientsParams{ProjectID: input.ID, Names: names}); err != nil {
			return err
		}
		for _, name := range names {
			if _, err := q.ActivateClientByName(ctx, sqlc.ActivateClientByNameParams{
				ID: uuid.New(), ProjectID: input.ID, Name: name,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return sqlc.Project{}, fmt.Errorf("update project with clients: %w", err)
	}
	return project, nil
}

func ReplaceUserProjects(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, projectIDs []uuid.UUID) error {
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return replaceUserProjects(ctx, sqlc.New(tx), userID, projectIDs)
	})
	if err != nil {
		return fmt.Errorf("replace user projects: %w", err)
	}
	return nil
}

func replaceUserProjects(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, projectIDs []uuid.UUID) error {
	if _, err := q.LockUserForProjectAssignment(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteUserProjects(ctx, userID); err != nil {
		return err
	}
	for _, id := range projectIDs {
		if err := q.AddUserToProject(ctx, sqlc.AddUserToProjectParams{UserID: userID, ProjectID: id}); err != nil {
			return err
		}
	}
	return nil
}
