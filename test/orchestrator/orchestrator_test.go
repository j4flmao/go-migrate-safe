package orchestrator_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/orchestrator"
	"github.com/j4flmao/go-migrate-safe/driver/memory"
)

func TestRunDiff_NoModels(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New(migrate.WithModels())
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "diff", opts)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunGenerate_NoChanges(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New(migrate.WithModels())
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "generate", opts)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if res.Pipeline != "ok" || !res.DiffPlan.IsEmpty {
		t.Errorf("expected empty plan, got %+v", res)
	}
}

func TestRunValidate_EmptyDir(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "validate", opts)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunStatus_Empty(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "status", opts)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunRollback_Empty(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "rollback", opts)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunHistory_Empty(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "history", opts)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunApply_EmptyDir(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "apply", opts)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunApplyDryRun_EmptyDir(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir(), DialectName: "postgres"}
	res, err := orchestrator.Run(context.Background(), "apply-dry-run", opts)
	if err != nil {
		t.Fatalf("apply-dry-run: %v", err)
	}
	if res.Pipeline != "ok" {
		t.Errorf("pipeline = %q, want ok", res.Pipeline)
	}
}

func TestRunUnknownIntent(t *testing.T) {
	d := memory.New()
	m, _ := migrate.New()
	opts := orchestrator.Options{Migrator: m, DBDriver: d, OutputDir: t.TempDir()}
	res, err := orchestrator.Run(context.Background(), "foobar", opts)
	if err == nil {
		t.Fatal("expected error for unknown intent")
	}
	if res != nil {
		t.Errorf("expected nil result for unknown intent")
	}
}
