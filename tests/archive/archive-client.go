package archive

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *ArchiveSuite) TestCreateArchiveForClient() {
	t := a.T()

	projectId := primitive.NewObjectID()
	clientID := "clientID"
	err := a.archiveService.CreateSetting(context.Background(), projectId.Hex(), clientID, 10)
	assert.NoError(t, err)

	tenWeeksAgo := time.Now().Add(-10*7*24*time.Hour - 1*time.Second)
	err = a.prepopulateLogs(10_000, tenWeeksAgo, projectId.Hex(), clientID, "::1")
	assert.NoError(t, err)

	err = a.prepopulateLogs(1000, time.Now(), projectId.Hex(), clientID, "::1")

	count, err := a.logCollection.CountDocuments(context.Background(), bson.M{
		"client.project_id": projectId.Hex(),
		"client.client_id":  clientID,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(11_000), count, "Should have 11,000 test logs")

	archiveResult, err := a.archiveService.CreateArhiveForClient(context.Background(), projectId.Hex(), clientID)
	assert.NoError(t, err)

	// assert only 1000 logs remain, all not older than 10 weeks
	count, err = a.logCollection.CountDocuments(context.Background(), bson.M{
		"client.project_id": projectId.Hex(),
		"client.client_id":  clientID,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), count, "Should have 1000 log lines remaining")

	// assert contents of generated zip
	zipBytes, err := a.filesService.GetFileById(context.Background(), archiveResult.FileID.Hex())
	assert.NoError(t, err)

	zipReader, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	assert.NoError(t, err)

	sumLines := 0

	for _, file := range zipReader.File {
		rc, err := file.Open()
		assert.NoError(t, err)
		defer rc.Close()

		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			sumLines++
		}
		assert.NoError(t, scanner.Err())
	}
	assert.Equal(t, 10_000, sumLines, "Should have 10,000 log lines in the zip")

	// Optionally, verify the age of remaining logs
	cursor, err := a.logCollection.Find(context.Background(), bson.M{
		"client.project_id": projectId.Hex(),
		"client.client_id":  clientID,
		"timestamp": bson.M{
			"$lt": tenWeeksAgo,
		},
	})
	assert.NoError(t, err)
	var remainingOldLogs []interface{}
	err = cursor.All(context.Background(), &remainingOldLogs)
	assert.NoError(t, err)
	assert.Len(t, remainingOldLogs, 0, "Should have no logs older than 10 weeks remaining")
}

func (a *ArchiveSuite) prepopulateLogs(n int, cuttoffDate time.Time, projectID string, clientID string, clientInstance string) error {
	logLines := make([]types.StoredLog, n)

	for i := 0; i < n; i++ {
		logLines[i] = types.StoredLog{
			Timestamp:      cuttoffDate.Add(-time.Duration(i) * time.Millisecond),
			SequenceNumber: i,
			LogLine:        "log line",
			Client: types.StoredClient{
				InstanceId: clientInstance,
				ProjectId:  projectID,
				ClientId:   clientID,
			},
		}
	}
	err := a.logService.SaveLogs(context.Background(), logLines)
	return err
}
