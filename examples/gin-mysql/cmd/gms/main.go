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
  seed        Seed initial data into database

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
