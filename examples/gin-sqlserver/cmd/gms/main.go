package main

import (
	"log"
	"os"

	"github.com/j4flmao/go-migrate-safe/examples/gin-sqlserver/internal"
	_ "github.com/microsoft/go-mssqldb"
)

const usage = `gms — go-migrate-safe example CLI

Usage:
  gms <command>

Commands:
  generate    Generate migration files from User/Product structs
  apply       Apply pending migrations
  seed        Seed initial data into database

Environment:
  SQLSERVER_DSN    SQL Server DSN (default: server=DESKTOP-HLUM9KN;database=go_migrate_example;trusted_connection=true)
`

func main() {
	if len(os.Args) < 2 {
		log.Print(usage)
		return
	}

	dsn := "server=DESKTOP-HLUM9KN;database=go_migrate_example;trusted_connection=true;trustServerCertificate=true"
	if v := os.Getenv("SQLSERVER_DSN"); v != "" {
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
