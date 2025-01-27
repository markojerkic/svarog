package archive

import (
	"context"

	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *ArchiveSuite) TestCreateSettings() {
	t := a.T()

	projectId := primitive.NewObjectID()
	clientID := "clientID"
	err := a.archiveService.CreateSetting(context.Background(), projectId.Hex(), clientID, 10)
	assert.NoError(t, err)
}

func (a *ArchiveSuite) TestCreateSettingsForExistingPair() {
	t := a.T()

	projectId := primitive.NewObjectID()
	clientID := "clientID"
	err := a.archiveService.CreateSetting(context.Background(), projectId.Hex(), clientID, 10)
	assert.NoError(t, err)

	err = a.archiveService.CreateSetting(context.Background(), projectId.Hex(), clientID, 10)
	assert.Error(t, err)
}

func (a *ArchiveSuite) TestUpdateSettings() {
	t := a.T()

	projectId := primitive.NewObjectID()
	clientID := "clientID"
	err := a.archiveService.CreateSetting(context.Background(), projectId.Hex(), clientID, 10)
	assert.NoError(t, err)
	var setting types.ArchiveSetting

	err = a.archiveSettingCollection.FindOne(context.Background(), bson.M{"client_id": clientID, "project_id": projectId}).Decode(&setting)
	assert.NoError(t, err)

	err = a.archiveService.UpdateSetting(context.Background(), setting.ID.Hex(), 20)
	assert.NoError(t, err)

	// assert only one setting exists
	count, err := a.archiveSettingCollection.CountDocuments(context.Background(), bson.M{"client_id": clientID, "project_id": projectId})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count, "There should be only one setting")

	// assert that the setting was updated
	err = a.archiveSettingCollection.FindOne(context.Background(), bson.M{"client_id": clientID, "project_id": projectId}).Decode(&setting)
	assert.NoError(t, err)

	assert.Equal(t, 20, setting.ArhiveAfterWeeks)
}

func (a *ArchiveSuite) TestUpdateNonExistingSettings() {
	t := a.T()

	err := a.archiveService.UpdateSetting(context.Background(), "clientID", 20)
	assert.Error(t, err)
}
