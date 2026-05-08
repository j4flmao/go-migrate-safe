package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func dsn() string {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		return v
	}
	return "ostgres://postgres:secret123@127.0.0.1:5435/go_migrate_example?sslmode=disable"
}

func main() {
	db, err := sql.Open("postgres", dsn())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v (is PostgreSQL running?)", err)
	}

	r := newRouter(db)
	addr := ":8083"
	if v := os.Getenv("LISTEN"); v != "" {
		addr = v
	}
	log.Printf("Listening on %s", addr)
	r.Run(addr)
}
