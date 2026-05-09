package store_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/sqlite"
	"github.com/j4flmao/go-migrate-safe/migrate"
	_ "github.com/mattn/go-sqlite3"
)

func TestStore_SQLiteHistory(t *testing.T) {
	dbPath := "test_store.db"
	defer os.Remove(dbPath)

	db, _ := sql.Open("sqlite3", dbPath)
	defer db.Close()

	drv := sqlite.New(db)
	ctx := context.Background()

	// 1. Ensure table
	if err := drv.EnsureHistoryTable(ctx); err != nil {
		t.Fatalf("EnsureHistoryTable: %v", err)
	}

	// 2. Record migration
	rec := migrate.MigrationRecord{
		Version:     1,
		Name:        "init",
		Direction:   "up",
		Checksum:    "hash1",
		Status:      "applied",
		AppliedAt:   "2024-01-01T00:00:00Z",
		ExecutionMS: 100,
	}
	if err := drv.RecordMigration(ctx, rec); err != nil {
		t.Fatalf("RecordMigration: %v", err)
	}

	// 3. Load history
	hist, err := drv.LoadHistory(ctx)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}

	if len(hist) != 1 {
		t.Fatalf("expected 1 record, got %d", len(hist))
	}
	if hist[0].Version != 1 || hist[0].Name != "init" || hist[0].Checksum != "hash1" {
		t.Errorf("record mismatch: %+v", hist[0])
	}

	// 4. Record failure
	failRec := migrate.MigrationRecord{
		Version:      2,
		Name:         "bad",
		Direction:    "up",
		Status:       "failed",
		ErrorMessage: "something went wrong",
	}
	drv.RecordMigration(ctx, failRec)

	hist, _ = drv.LoadHistory(ctx)
	if len(hist) != 2 {
		t.Errorf("expected 2 records, got %d", len(hist))
	}
}
