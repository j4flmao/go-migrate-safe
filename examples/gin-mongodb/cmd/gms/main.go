package main

import (
	"log"
	"os"

	"github.com/j4flmao/go-migrate-safe/examples/gin-mongodb/internal"
)

const usage = `gms — go-migrate-safe MongoDB example CLI

Usage:
  gms <command>

Commands:
  generate    Generate .json migration files from struct models
  apply       Apply pending migrations
  seed        Seed initial data into database

Environment:
  MONGODB_URI        MongoDB URI (default: mongodb://localhost:27017)
  MONGODB_DATABASE   Database name (default: go_migrate_example)
`

func main() {
	if len(os.Args) < 2 {
		log.Print(usage)
		return
	}

	uri := "mongodb://localhost:27017"
	if v := os.Getenv("MONGODB_URI"); v != "" {
		uri = v
	}

	switch os.Args[1] {
	case "generate":
		internal.MigrateRun("generate", uri)
	case "apply":
		internal.MigrateRun("apply", uri)
	case "seed":
		internal.SeedRun(uri)
	default:
		log.Printf("unknown command: %q", os.Args[1])
		log.Print(usage)
		os.Exit(1)
	}
}
