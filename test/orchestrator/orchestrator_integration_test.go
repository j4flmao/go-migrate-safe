package orchestrator_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/memory"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/orchestrator"
	"github.com/j4flmao/go-migrate-safe/parser"
)

type User struct {
	ID   int64  `db:"id,pk,autoincrement"`
	Name string `db:"name,not null"`
}

func TestOrchestrator_FullCycle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	drv := memory.New()

	// 1. Initial Generate
	m, _ := migrate.New(migrate.WithModels(User{}), migrate.WithOutputDir(dir))
	opts := orchestrator.Options{
		Migrator: m, DBDriver: drv, Models: []any{User{}},
		OutputDir: dir, DialectName: "postgres", NameOverride: "init",
	}

	res, err := orchestrator.Run(ctx, "generate", opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(res.GeneratedFiles) != 2 {
		t.Errorf("expected 2 files, got %v", res.GeneratedFiles)
	}

	// 2. Apply
	res, err = orchestrator.Run(ctx, "apply", opts)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// 3. Check status
	res, err = orchestrator.Run(ctx, "status", opts)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("status failed: %v", res.Pipeline)
	}

	// 4. Change model and generate again
	type UserV2 struct {
		_     struct{} `db:"table:users"`
		ID    int64    `db:"id,pk,autoincrement"`
		Name  string   `db:"name,not null"`
		Email string   `db:"email,unique"`
	}

	m2, _ := migrate.New(migrate.WithModels(UserV2{}), migrate.WithOutputDir(dir))
	opts2 := orchestrator.Options{
		Migrator: m2, DBDriver: drv, Models: []any{UserV2{}},
		OutputDir: dir, DialectName: "postgres", NameOverride: "add_email",
	}

	res, err = orchestrator.Run(ctx, "generate", opts2)
	if err != nil {
		t.Fatalf("generate v2: %v", err)
	}
	if res.DiffPlan.IsEmpty {
		t.Fatal("expected changes in v2")
	}

	// 5. Apply v2
	res, err = orchestrator.Run(ctx, "apply", opts2)
	if err != nil {
		t.Fatalf("apply v2: %v", err)
	}

	// 6. Rollback
	res, err = orchestrator.Run(ctx, "rollback", opts2)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
}

func TestOrchestrator_ValidationFail(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	drv := memory.New()

	type Product struct {
		ID int64 `db:"id,pk"`
	}
	type Other struct {
		ID int64 `db:"id,pk"`
	}

	// Initial state: both tables exist
	m1, _ := migrate.New(migrate.WithModels(Product{}, Other{}), migrate.WithOutputDir(dir))
	opts1 := orchestrator.Options{Migrator: m1, DBDriver: drv, Models: []any{Product{}, Other{}}, OutputDir: dir, DialectName: "postgres"}
	orchestrator.Run(ctx, "generate", opts1)
	orchestrator.Run(ctx, "apply", opts1)

	// Manually seed the driver because memory driver doesn't auto-update schema from SQL
	p := parser.New(parser.DialectPostgres, "public")
	smDB, _ := p.Parse(Product{}, Other{})
	drv.SeedSchema(smDB)

	// State 2: Product removed (destructive)
	m2, _ := migrate.New(migrate.WithModels(Other{}), migrate.WithOutputDir(dir))
	opts2 := orchestrator.Options{Migrator: m2, DBDriver: drv, Models: []any{Other{}}, OutputDir: dir, DialectName: "postgres"}

	res, err := orchestrator.Run(ctx, "generate", opts2)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Validation should fail because safety options don't allow drop table
	if len(res.Errors) == 0 {
		t.Error("expected validation error for drop table")
	}
}
