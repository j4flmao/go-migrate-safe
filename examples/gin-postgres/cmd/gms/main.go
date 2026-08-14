package main

import (
	"log"
	"os"

	"github.com/j4flmao/go-migrate-safe/examples/gin-postgres/internal"
	_ "github.com/lib/pq"
)

const usage = `gms — go-migrate-safe PostgreSQL example CLI

Usage:
  gms <command>

Commands:
  generate    Generate migration files from struct models
  apply       Apply pending migrations
  status      Show current migration status
  history     Show migration history
  validate    Validate migration files
  rollback    Rollback last migration
  diff        Show what would change (no files written)
  seed        Seed initial data into database
  doctor      Diagnose database connection and health
  studio      Launch GMS Studio web UI to browse the database

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
	case "generate", "apply", "status", "history", "validate", "rollback", "doctor", "diff":
		internal.MigrateRun(os.Args[1], dsn)
	case "seed":
		internal.SeedRun(dsn)
	case "studio":
		addr := "127.0.0.1:4488"
		if v := os.Getenv("STUDIO_ADDR"); v != "" {
			addr = v
		}
		open := os.Getenv("STUDIO_NO_OPEN") == ""
		internal.StudioRun(dsn, addr, open)
	default:
		log.Printf("unknown command: %q", os.Args[1])
		log.Print(usage)
		os.Exit(1)
	}
}
