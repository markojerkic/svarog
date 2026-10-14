package db

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

func TestSaveLogsRejectsInvalidClientIDBeforeDatabaseAccess(t *testing.T) {
	service := &MongoLogService{}

	err := service.SaveLogs(context.Background(), []types.StoredLog{{
		Client: types.StoredClient{ClientId: "mongo-project"},
	}})

	require.Error(t, err, "legacy client ID must be rejected before database access")

	err = service.SaveLogs(context.Background(), []types.StoredLog{{
		Client: types.StoredClient{ClientId: uuid.Nil.String()},
	}})

	require.Error(t, err, "nil client UUID must be rejected before database access")
}
