package internal

import (
	"context"
	"database/sql"
	"log"

	"github.com/j4flmao/go-migrate-safe/driver/mssql"
	"github.com/j4flmao/go-migrate-safe/studio"
)

// StudioRun launches the GMS Studio web UI against the configured SQL Server.
func StudioRun(dsn, addr string, openBrowser bool) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v (is SQL Server reachable?)", err)
	}

	drv := mssql.New(db)
	srv, err := studio.New(studio.Options{
		DB:          db,
		DriverName:  "mssql",
		Schema:      "dbo",
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
