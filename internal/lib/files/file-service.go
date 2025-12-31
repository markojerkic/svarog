package files

import (
	"context"
	"errors"
	"os"
	"time"

	"log/slog"
	"github.com/markojerkic/svarog/internal/server/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FileService interface {
	SaveFile(ctx context.Context, name string, path string) (primitive.ObjectID, error)
	GetFile(ctx context.Context, name string) ([]byte, error)
	GetFileById(ctx context.Context, id string) ([]byte, error)
	DeleteFile(ctx context.Context, id primitive.ObjectID) error
}

type FileServiceImpl struct {
	fileCollection *mongo.Collection
}

const (
	ErrFileNotFound = "File not found"
)

// GetFile implements FileService.
func (f *FileServiceImpl) GetFile(ctx context.Context, name string) ([]byte, error) {
	var savedFile types.SavedFile
	err := f.fileCollection.FindOne(ctx, bson.M{
		"name": name,
	}).Decode(&savedFile)
	if err != nil {
		slog.Error("Error reading file", "err", err)
		return nil, errors.New(ErrFileNotFound)
	}

	return savedFile.File, nil
}

// GetFileById implements FileService.
func (f *FileServiceImpl) GetFileById(ctx context.Context, id string) ([]byte, error) {
	var savedFile types.SavedFile
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.Join(errors.New("Error converting id to ObjectID"), err)
	}

	err = f.fileCollection.FindOne(ctx, bson.M{
		"_id": oid,
	}).Decode(&savedFile)
	if err != nil {
		return nil, errors.Join(errors.New("Error finding file"), err)
	}

	return savedFile.File, nil
}

// DeleteFile implements FileService.
func (f *FileServiceImpl) DeleteFile(ctx context.Context, id primitive.ObjectID) error {
	_, err := f.fileCollection.DeleteOne(ctx, bson.M{
		"_id": id,
	})
	return err
}

// SaveFile implements FileService.
func (f *FileServiceImpl) SaveFile(ctx context.Context, name string, path string) (primitive.ObjectID, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		slog.Error("Error reading file", "err", err)
		return primitive.NilObjectID, errors.New(ErrFileNotFound)
	}

	result, err := f.fileCollection.InsertOne(ctx, types.SavedFile{
		File:      file,
		Name:      name,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
	})

	return result.InsertedID.(primitive.ObjectID), err
}

var _ FileService = &FileServiceImpl{}

func NewFileService(fileCollection *mongo.Collection) FileService {
	return &FileServiceImpl{fileCollection: fileCollection}
}
