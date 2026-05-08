package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/microsoft/go-mssqldb"
)

func dsn() string {
	if v := os.Getenv("SQLSERVER_DSN"); v != "" {
		return v
	}
	return "server=DESKTOP-HLUM9KN;database=go_migrate_example;trusted_connection=true;trustServerCertificate=true"
}

func main() {
	db, err := sql.Open("sqlserver", dsn())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v (is SQL Server running?)", err)
	}

	r := newRouter(db)
	addr := ":8082"
	if v := os.Getenv("LISTEN"); v != "" {
		addr = v
	}
	log.Printf("Listening on %s", addr)
	r.Run(addr)
}
