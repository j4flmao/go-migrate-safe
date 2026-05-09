package executor_test

import (
	"context"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/driver/memory"
	"github.com/j4flmao/go-migrate-safe/executor"
	"github.com/j4flmao/go-migrate-safe/store"
)

func TestExecutor_Apply_Success(t *testing.T) {
	drv := memory.New()
	exec := executor.New(drv)
	exec.Now = func() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

	files := []store.File{
		{
			Version:   1,
			Name:      "init",
			Direction: "up",
			Format:    "sql",
			Body:      "CREATE TABLE users (id BIGINT);",
			Checksum:  "abc",
		},
	}

	res, err := exec.Apply(context.Background(), files)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(res.Applied) != 1 {
		t.Errorf("expected 1 applied file, got %d", len(res.Applied))
	}

	history, _ := drv.LoadHistory(context.Background())
	if len(history) != 1 || history[0].Status != "applied" {
		t.Errorf("history record wrong: %+v", history)
	}
}

func TestExecutor_Apply_Failure(t *testing.T) {
	drv := memory.New()
	exec := executor.New(drv)
	files := []store.File{
		{
			Version:   1,
			Name:      "init",
			Direction: "up",
			Format:    "sql",
			Body:      "INVALID SQL;",
			Checksum:  "abc",
		},
	}

	res, err := exec.Apply(context.Background(), files)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if res.Failed == nil || res.Failed.Version != 1 {
		t.Errorf("expected failed file v1, got %v", res.Failed)
	}

	history, _ := drv.LoadHistory(context.Background())
	if len(history) != 1 || history[0].Status != "failed" {
		t.Errorf("history record should be failed: %+v", history)
	}
}

func TestExecutor_DryRun(t *testing.T) {
	drv := memory.New()
	exec := executor.New(drv)
	exec.DryRun = true

	files := []store.File{
		{
			Version:   1,
			Name:      "init",
			Direction: "up",
			Format:    "sql",
			Body:      "CREATE TABLE users (id BIGINT);",
			Checksum:  "abc",
		},
	}

	_, err := exec.Apply(context.Background(), files)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// In dry run, we still record the migration as "applied" in some implementations,
	// or we don't. Let's check executor.go logic.
	// Current executor.go records it as "applied" even if DryRun=true (only tx.Exec is skipped).
	history, _ := drv.LoadHistory(context.Background())
	if len(history) != 1 || history[0].Status != "applied" {
		t.Errorf("history record should exist even in dry run: %+v", history)
	}
}
