package projects

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/server/db"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ProjectsService interface {
	CreateProject(ctx context.Context, name string, clients []string) (Project, error)
	GetProject(ctx context.Context, id string) (Project, error)
	GetProjects(ctx context.Context) ([]Project, error)
	GetProjectByClient(ctx context.Context, client string) (Project, error)
	DeleteProject(ctx context.Context, id string) error
	RemoveClientFromProject(ctx context.Context, projectId string, client string) error
	AddClientToProject(ctx context.Context, projectId string, client string) error
	GetStorageSizeForProject(ctx context.Context, projectId string) (float64, error)
}

type MongoProjectsService struct {
	mongoClient        *mongo.Client
	projectsCollection *mongo.Collection
	logsService        db.LogRepository
}

const (
	ErrProjectNotFound = "project not found"
	ErrClientNotFound  = "client not found"
	ErrProjectExists   = "project already exists"
)

// CreateProject implements ProjectsService.
func (m *MongoProjectsService) CreateProject(ctx context.Context, name string, clients []string) (Project, error) {
	result, err := m.projectsCollection.InsertOne(ctx, bson.M{"name": name, "clients": clients})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return Project{}, errors.New(ErrProjectExists)
		}

		log.Error("Error creating project", "error", err)
		return Project{}, err
	}

	return Project{
		ID:      result.InsertedID.(primitive.ObjectID),
		Name:    name,
		Clients: uniqueStrings(clients),
	}, nil
}

// GetProject implements ProjectsService.
func (m *MongoProjectsService) GetProject(ctx context.Context, id string) (Project, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return Project{}, err
	}
	var project Project
	err = m.projectsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&project)
	if err != nil {
		return Project{}, errors.New(ErrProjectNotFound)
	}
	return project, nil
}

func (m *MongoProjectsService) GetProjects(ctx context.Context) ([]Project, error) {
	pipeline := mongo.Pipeline{
		// Lookup stage
		{
			{Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "log_lines"},
				{Key: "localField", Value: "clients"},
				{Key: "foreignField", Value: "client.client_id"},
				{Key: "as", Value: "log_lines"},
			}},
		},
		// Add fields stage
		{
			{Key: "$addFields", Value: bson.D{
				{Key: "totalSizeBytes", Value: bson.D{
					{Key: "$sum", Value: bson.D{
						{Key: "$map", Value: bson.D{
							{Key: "input", Value: "$log_lines"},
							{Key: "as", Value: "log_line"},
							{Key: "in", Value: bson.D{
								{Key: "$bsonSize", Value: "$$log_line"},
							}},
						}},
					}},
				}},
			}},
		},
		// Project stage
		{
			{Key: "$project", Value: bson.D{
				{Key: "_id", Value: 1},
				{Key: "name", Value: 1},
				{Key: "clients", Value: 1},
				{Key: "totalSizeMB", Value: bson.D{
					{Key: "$round", Value: bson.A{
						bson.D{
							{Key: "$divide", Value: bson.A{"$totalSizeBytes", 1024 * 1024}},
						},
						2,
					}},
				}},
			}},
		},
	}

	// Execute aggregation
	cursor, err := m.projectsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to execute aggregation: %w", err)
	}
	defer cursor.Close(ctx)

	// Decode results
	var projects []Project
	if err := cursor.All(ctx, &projects); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	return projects, nil
}

// GetProjectByClient implements ProjectsService.
func (m *MongoProjectsService) GetProjectByClient(ctx context.Context, client string) (Project, error) {
	var project Project
	err := m.projectsCollection.FindOne(ctx, bson.M{
		"clients": bson.M{
			"$in": []string{client},
		},
	}).Decode(&project)
	if err != nil {
		return Project{}, errors.New(ErrProjectNotFound)
	}
	return project, nil
}

// AddClientToProject implements ProjectsService.
func (m *MongoProjectsService) AddClientToProject(ctx context.Context, projectId string, client string) error {
	_, err := util.StartTransaction(ctx, func(c mongo.SessionContext) (interface{}, error) {
		project, err := m.GetProject(ctx, projectId)
		if err != nil {
			return struct{}{}, err
		}
		clients := uniqueStrings(append(project.Clients, client))
		_, err = m.projectsCollection.UpdateOne(ctx, bson.M{"_id": project.ID}, bson.M{"$set": bson.M{"clients": clients}})
		return struct{}{}, err

	}, m.mongoClient)

	return err
}

// DeleteProject implements ProjectsService.
func (m *MongoProjectsService) DeleteProject(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = m.projectsCollection.DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil && err == mongo.ErrNoDocuments {
		return errors.New(ErrProjectNotFound)
	}

	return err
}

// RemoveClientFromProject implements ProjectsService.
func (m *MongoProjectsService) RemoveClientFromProject(ctx context.Context, projectId string, client string) error {
	objID, err := primitive.ObjectIDFromHex(projectId)
	if err != nil {
		return err
	}

	_, err = m.projectsCollection.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$pull": bson.M{"clients": client}})
	if err != nil && err == mongo.ErrNoDocuments {
		return errors.New(ErrProjectNotFound)
	}
	return err
}

func uniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func (m *MongoProjectsService) assertUniqeIndex(ctx context.Context) error {
	// Project name should be unique
	_, err := m.projectsCollection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{
			"name": 1,
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return errors.Join(errors.New("error creating unique index on project name"), err)
	}

	return nil
}

func (m *MongoProjectsService) GetStorageSizeForProject(ctx context.Context, projectId string) (float64, error) {
	project, err := m.GetProject(ctx, projectId)
	if err != nil {
		log.Error("Error getting project", "error", err)
		return 0, err
	}
	return m.logsService.GetStorageSizeForClients(ctx, project.Clients)
}

var _ ProjectsService = &MongoProjectsService{}

func NewProjectsService(projectsCollection *mongo.Collection, logsService db.LogRepository, mongoClient *mongo.Client) ProjectsService {
	service := &MongoProjectsService{
		projectsCollection: projectsCollection,
		mongoClient:        mongoClient,
		logsService:        logsService,
	}

	err := service.assertUniqeIndex(context.Background())
	if err != nil {
		log.Fatal("Error creating unique index", "error", err)
	}

	return service
}
