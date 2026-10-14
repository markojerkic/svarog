package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ArchiveEntry struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FileID    primitive.ObjectID `bson:"file_id" json:"fileId"`
	File      SavedFile          `bson:"file,omitempty" json:"file"`
	CreatedAt primitive.DateTime `bson:"created_at"`
	FromDate  primitive.DateTime `bson:"from_date"`
	ToDate    primitive.DateTime `bson:"to_date"`
}

type ArchiveSetting struct {
	ClientID         string
	ProjectID        string
	ArhiveAfterWeeks int
	UpdatedAt        time.Time
}
