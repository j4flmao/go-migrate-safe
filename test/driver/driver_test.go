package driver_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/driver/memory"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestMemoryDriver_ReadSchema_Empty(t *testing.T) {
	d := memory.New()
	sm, err := d.ReadSchema(context.Background(), "public")
	if err != nil {
		t.Fatalf("ReadSchema: %v", err)
	}
	if sm.Schema != "public" {
		t.Errorf("Schema = %q", sm.Schema)
	}
	if len(sm.Tables) != 0 {
		t.Errorf("Tables = %v", sm.Tables)
	}
}

func TestMemoryDriver_ReadSchema_Seeded(t *testing.T) {
	d := memory.New()
	tbl := migrate.NewTableModel("users")
	tbl.Columns["id"] = &migrate.ColumnModel{Name: "id", SQLType: "BIGINT", IsPK: true}
	sm := migrate.NewSchemaModel("public")
	sm.Tables["users"] = tbl
	d.SeedSchema(sm)

	got, _ := d.ReadSchema(context.Background(), "public")
	if _, ok := got.Tables["users"]; !ok {
		t.Fatal("users table missing after seed")
	}
}

func TestMemoryDriver_Exec(t *testing.T) {
	d := memory.New()
	err := d.Exec(context.Background(), "CREATE TABLE users (id BIGINT);")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(d.ExecLog) != 1 {
		t.Errorf("ExecLog len = %d", len(d.ExecLog))
	}
}

func TestMemoryDriver_Lock(t *testing.T) {
	d := memory.New()
	if err := d.AcquireLock(context.Background()); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if d.Locked != true {
		t.Error("Locked = false after AcquireLock")
	}
	if err := d.ReleaseLock(context.Background()); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}
	if d.Locked != false {
		t.Error("Locked = true after ReleaseLock")
	}
}

func TestMemoryDriver_LockTimeout(t *testing.T) {
	d := memory.New()
	_ = d.AcquireLock(context.Background())
	err := d.AcquireLock(context.Background())
	if err == nil {
		t.Fatal("expected lock timeout error")
	}
	_ = d.ReleaseLock(context.Background())
}

func TestMemoryDriver_History(t *testing.T) {
	d := memory.New()
	if err := d.EnsureHistoryTable(context.Background()); err != nil {
		t.Fatalf("EnsureHistoryTable: %v", err)
	}
	hist, err := d.LoadHistory(context.Background())
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(hist) != 0 {
		t.Errorf("History len = %d", len(hist))
	}
}

func TestMemoryDriver_RecordMigration(t *testing.T) {
	d := memory.New()
	rec := driver.MigrationRecord{
		Version:   1,
		Name:      "init",
		Direction: "up",
		Status:    "applied",
	}
	if err := d.RecordMigration(context.Background(), rec); err != nil {
		t.Fatalf("RecordMigration: %v", err)
	}
	hist, _ := d.LoadHistory(context.Background())
	if len(hist) != 1 {
		t.Fatalf("History len = %d", len(hist))
	}
}

func TestMemoryDriver_Close(t *testing.T) {
	d := memory.New()
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
