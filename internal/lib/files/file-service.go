package files

import (
	"context"
	"errors"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FileService interface {
	SaveFile(ctx context.Context, name string, path string) error
	GetFile(ctx context.Context, name string) ([]byte, error)
	GetFileById(ctx context.Context, id string) ([]byte, error)
}

type FileServiceImpl struct {
	fileCollection *mongo.Collection
}

// GetFile implements FileService.
func (f *FileServiceImpl) GetFile(ctx context.Context, name string) ([]byte, error) {
	var savedFile SavedFile
	err := f.fileCollection.FindOne(ctx, bson.M{
		"name": name,
	}).Decode(&savedFile)
	if err != nil {
		return nil, errors.Join(errors.New("Error finding file"), err)
	}

	return savedFile.File, nil
}

// GetFileById implements FileService.
func (f *FileServiceImpl) GetFileById(ctx context.Context, id string) ([]byte, error) {
	var savedFile SavedFile
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

// SaveFile implements FileService.
func (f *FileServiceImpl) SaveFile(ctx context.Context, name string, path string) error {
	file, err := os.ReadFile(path)
	if err != nil {
		return errors.Join(errors.New("Error reading file"), err)
	}

	_, err = f.fileCollection.InsertOne(ctx, SavedFile{
		File: file,
		Name: name,
	})

	return err
}

var _ FileService = &FileServiceImpl{}

func NewFileService(fileCollection *mongo.Collection) FileService {
	return &FileServiceImpl{fileCollection: fileCollection}
}
