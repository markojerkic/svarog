package db

import (
	"context"
	"fmt"

	rpc "github.com/markojerkic/svarog/proto"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LogServer struct {
	mongoClient *mongo.Client
}

func NewLogServer() *LogServer {
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI("mongodb://user:pass@localhost:27017"))

	if err != nil {
		panic(err)
	}

	return &LogServer{
		mongoClient: client,
	}
}

func (self *LogServer) Run(lines chan *rpc.LogLine) {
	for {
		select {
		case line := <-lines:
			fmt.Printf("Received log line: %v\n", line)
			// fmt.Printf("Received log line: %v\n", line)
			collection := self.mongoClient.Database("logs").Collection("log_lines")
			_, err := collection.InsertOne(context.Background(), line)
			if err != nil {
				panic(err)
			}
		}
	}
}
