package db

import (
	"context"
	"log"
	"testing"

	"github.com/markojerkic/svarog/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
)

func TestMongoLogsRepository(t *testing.T) {
	log.Println("TestMongoLogsRepository")
	ctx := context.Background()

	mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	if err != nil {
		log.Fatalf("failed to start container: %s", err)
	}

	// Clean up the container
	defer func() {
		if err := mongodbContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}
	}()

	mongoContainerConnectionString, err := mongodbContainer.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("failed to get container connection string: %s", err)
	}

	mongoRepository := db.NewMongoClient(mongoContainerConnectionString)
	// mongoServer := db.NewLogServer(mongoRepository)

	clients, err := mongoRepository.GetClients()
	if err != nil {
		t.Fatalf("Error getting clients: %v", err)
	}

	if len(clients) != 0 {
		t.Fatalf("Expected 0 clients, got %d", len(clients))
	}

}
