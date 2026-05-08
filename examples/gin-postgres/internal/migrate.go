package internal

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    "github.com/j4flmao/go-migrate-safe/driver/postgres"
    "github.com/j4flmao/go-migrate-safe/migrate"
    "github.com/j4flmao/go-migrate-safe/orchestrator"
    _ "github.com/lib/pq"
)

func MigrateRun(intent, dsn string) {
    ctx := context.Background()

    db, err := sql.Open("postgres", dsn)
    if err != nil {
        log.Fatalf("db: %v", err)
    }
    defer db.Close()

    if err := db.PingContext(ctx); err != nil {
        log.Fatalf("db: %v (is PostgreSQL running?)", err)
    }

    if err := db.PingContext(ctx); err != nil {
        log.Fatalf("db: %v (is PostgreSQL running?)", err)
    }

    drv := postgres.New(db)

	m, err := migrate.New(
        migrate.WithModels(Models()...),
        migrate.WithOutputDir("./migrations"),
        migrate.WithDriver("postgres"),
    )
    if err != nil {
        log.Fatalf("migrate: %v", err)
    }

    res, err := orchestrator.Run(ctx, intent, orchestrator.Options{
        Migrator:    m,
        DBDriver:    drv,
        OutputDir:   "./migrations",
        DialectName: "postgres",
    })
    if err != nil {
        log.Fatalf("%s: %v", intent, err)
    }
    if res.Explain != "" {
        fmt.Print(res.Explain)
    }
    for _, w := range res.Warnings {
        log.Printf("WARNING [%s]: %s", w.Code, w.Message)
    }
    for _, e := range res.Errors {
        log.Printf("ERROR [%s]: %s", e.Code, e.Message)
        if e.Suggestion != "" {
            log.Printf("  suggestion: %s", e.Suggestion)
        }
    }
}