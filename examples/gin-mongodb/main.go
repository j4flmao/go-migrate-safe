package main

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	uri := "mongodb://localhost:27017"
	if v := os.Getenv("MONGODB_URI"); v != "" {
		uri = v
	}
	dbName := "go_migrate_example"
	if v := os.Getenv("MONGODB_DATABASE"); v != "" {
		dbName = v
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("mongodb: %v", err)
	}
	defer client.Disconnect(context.Background())

	if err := client.Ping(context.Background(), nil); err != nil {
		log.Fatalf("mongodb: ping: %v (is MongoDB running?)", err)
	}
	log.Print("MongoDB connected")

	db := client.Database(dbName)
	r := newRouter(db)

	addr := ":8082"
	if v := os.Getenv("LISTEN"); v != "" {
		addr = v
	}
	log.Printf("Listening on %s", addr)
	r.Run(addr)
}
