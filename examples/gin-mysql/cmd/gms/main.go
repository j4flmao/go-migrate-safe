package main

import (
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/j4flmao/go-migrate-safe/examples/gin-mysql/internal"
)

const usage = `gms — go-migrate-safe example CLI

Usage:
  gms <command>

Commands:
  generate    Generate migration files from User/Product structs
  apply       Apply pending migrations
  status      Show current migration status
  history     Show migration history
  validate    Validate migration files
  rollback    Rollback last migration
  seed        Seed initial data into database
  studio      Launch GMS Studio web UI to browse the database

Environment:
  MYSQL_DSN    MySQL DSN (default: root@tcp(127.0.0.1:3306)/go_migrate_example)
`

func main() {
	if len(os.Args) < 2 {
		log.Print(usage)
		return
	}

	dsn := "root@tcp(127.0.0.1:3306)/go_migrate_example?parseTime=true&charset=utf8mb4"
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		dsn = v
	}

	switch os.Args[1] {
	case "generate", "apply", "status", "history", "validate", "rollback":
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
