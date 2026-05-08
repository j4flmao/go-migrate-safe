package main

import (
	"log"
	"os"

	_ "github.com/lib/pq"
	"github.com/j4flmao/go-migrate-safe/examples/gin-postgres/internal"
)

const usage = `gms — go-migrate-safe PostgreSQL example CLI

Usage:
  gms <command>

Commands:
  generate    Generate migration files from struct models
  apply       Apply pending migrations
  seed        Seed initial data into database

Environment:
  POSTGRES_DSN   PostgreSQL DSN (default: postgres://postgres:secret123@127.0.0.1:5435/go_migrate_example?sslmode=disable)
`

func main() {
	if len(os.Args) < 2 {
		log.Print(usage)
		return
	}

	dsn := "postgres://postgres:secret123@127.0.0.1:5435/go_migrate_example?sslmode=disable"
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		dsn = v
	}

	switch os.Args[1] {
	case "generate":
		internal.MigrateRun("generate", dsn)
	case "apply":
		internal.MigrateRun("apply", dsn)
	case "seed":
		internal.SeedRun(dsn)
	default:
		log.Printf("unknown command: %q", os.Args[1])
		log.Print(usage)
		os.Exit(1)
	}
}
