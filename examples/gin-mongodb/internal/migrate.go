package internal

import (
	"context"
	"fmt"
	"log"

	"github.com/j4flmao/go-migrate-safe/driver/mongodb"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/orchestrator"
)

func MigrateRun(intent, uri string) {
	ctx := context.Background()

	drv, err := mongodb.New(ctx, uri, "go_migrate_example")
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer drv.Close()

	m, err := migrate.New(
		migrate.WithModels(Models()...),
		migrate.WithOutputDir("./migrations"),
		migrate.WithDriver("mongodb"),
	)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	res, err := orchestrator.Run(ctx, intent, orchestrator.Options{
		Migrator:    m,
		DBDriver:    drv,
		OutputDir:   "./migrations",
		DialectName: "mongodb",
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
	if len(res.GeneratedFiles) > 0 {
		log.Printf("Generated: %v", res.GeneratedFiles)
	}
}
