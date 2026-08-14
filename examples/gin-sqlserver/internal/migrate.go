package internal

import (
	"context"
	"database/sql"
	"log"

	"github.com/j4flmao/go-migrate-safe/driver/mssql"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/orchestrator"
)

func MigrateRun(intent, dsn string) {
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v", err)
	}

	drv := mssql.New(db)
	m, err := migrate.New(
		migrate.WithModels(Models()...),
		migrate.WithOutputDir("./migrations"),
		migrate.WithSchema("dbo"),
		migrate.WithDriver("mssql"),
		migrate.WithAllowTypeChange(migrate.TypeChangeAny),
		migrate.WithAllowDropTable(),
	)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	res, err := orchestrator.Run(ctx, intent, orchestrator.Options{
		Migrator:    m,
		DBDriver:    drv,
		OutputDir:   "./migrations",
		DialectName: "mssql",
	})
	if err != nil {
		log.Fatalf("%s: %v", intent, err)
	}
	if res.Explain != "" {
		log.Print(res.Explain)
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
	if len(res.GeneratedFiles) > 0 {
		log.Printf("Generated: %v", res.GeneratedFiles)
	}
}
