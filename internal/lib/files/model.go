package files

import "go.mongodb.org/mongo-driver/bson/primitive"

type SavedFile struct {
	ID   primitive.ObjectID `bson:"_id"`
	Name string             `bson:"name"`
	File []byte             `bson:"file"`
}
