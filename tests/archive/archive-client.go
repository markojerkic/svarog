package archive

import (
	"context"
	"time"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *ArchiveSuite) TestCreateArchiveForClient() {
	t := a.T()

	projectId := primitive.NewObjectID()
	clientID := "clientID"
	err := a.archiveService.CreateSetting(context.Background(), projectId.Hex(), clientID, 10)
	assert.NoError(t, err)

	tenWeeksAgo := time.Now().Add(-10*7*24*time.Hour + 1*time.Second)
	err = a.prepopulateLogs(10_000, tenWeeksAgo, clientID, "::1")
	assert.NoError(t, err)

	err = a.prepopulateLogs(1000, time.Now(), clientID, "::1")

	_, err = a.archiveService.CreateArhiveForClient(context.Background(), projectId.Hex(), clientID)
	assert.NoError(t, err)

}

func (a *ArchiveSuite) prepopulateLogs(n int, cuttoffDate time.Time, clientID string, clientInstance string) error {
	logLines := make([]types.StoredLog, n)

	for i := 0; i < n; i++ {
		logLines[i] = types.StoredLog{
			Timestamp:      cuttoffDate.Add(-time.Duration(i) * time.Millisecond),
			SequenceNumber: int64(i),
			LogLine:        "log line",
			Client: types.StoredClient{
				IpAddress: clientInstance,
				ClientId:  clientID,
			},
		}
	}
	err := a.logService.SaveLogs(context.Background(), logLines)
	return err
}
