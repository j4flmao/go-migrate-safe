package internal

import (
	"context"
	"log"
	"time"

	"github.com/j4flmao/go-migrate-safe/driver/mongodb"
	"github.com/j4flmao/go-migrate-safe/studio"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func StudioRun(uri, addr string, openBrowser bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v (is MongoDB running?)", err)
	}

	db := client.Database("go_migrate_example")
	drv, err := mongodb.New(ctx, uri, "go_migrate_example")
	if err != nil {
		log.Fatalf("mongo driver: %v", err)
	}

	srv, err := studio.New(studio.Options{
		MongoDB:     db,
		DriverName:  "mongodb",
		Schema:      "go_migrate_example",
		Driver:      drv,
		OpenBrowser: openBrowser,
	})
	if err != nil {
		log.Fatalf("studio init: %v", err)
	}

	if err := srv.Listen(addr); err != nil {
		log.Fatalf("studio: %v", err)
	}
}
