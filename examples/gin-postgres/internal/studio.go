package internal

import (
	"context"
	"database/sql"
	"log"

	"github.com/j4flmao/go-migrate-safe/driver/postgres"
	"github.com/j4flmao/go-migrate-safe/studio"
	_ "github.com/lib/pq"
)

// StudioRun launches the GMS Studio web UI against the configured PostgreSQL database.
func StudioRun(dsn, addr string, openBrowser bool) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v (is PostgreSQL running?)", err)
	}

	drv := postgres.New(db)
	srv, err := studio.New(studio.Options{
		DB:          db,
		DriverName:  "postgres",
		Schema:      "public",
		Driver:      drv,
		OpenBrowser: openBrowser,
	})
	if err != nil {
		log.Fatalf("studio: %v", err)
	}
	if err := srv.Listen(addr); err != nil {
		log.Fatalf("studio: %v", err)
	}
}
