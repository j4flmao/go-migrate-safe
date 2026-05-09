package codegen_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/codegen"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func strPtr(s string) *string { return &s }

func makeTable() *migrate.TableModel {
	t := migrate.NewTableModel("orders")
	t.Columns["id"] = &migrate.ColumnModel{Name: "id", SQLType: "BIGINT", IsPK: true, AutoIncrement: true}
	t.Columns["user_id"] = &migrate.ColumnModel{Name: "user_id", SQLType: "BIGINT"}
	t.Columns["status"] = &migrate.ColumnModel{Name: "status", SQLType: "VARCHAR(50)", Default: strPtr("'pending'")}
	t.ColumnOrder = []string{"id", "user_id", "status"}
	return t
}

func fixedNow() time.Time {
	return time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
}

func TestCodegen_Postgres_AddTable_GoldenLikeShape(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1,
		Name:    "add_orders_table",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "orders", NewTable: makeTable(), Reason: "Added orders table"},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	res, err := g.Generate(plan)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	must := []string{
		"-- go-migrate-safe v0.1.0",
		"-- Version: 1",
		"-- Direction: UP",
		"-- Checksum: sha256:",
		"CREATE TABLE IF NOT EXISTS orders",
		"BIGSERIAL",
		"PRIMARY KEY",
		"DEFAULT 'pending'",
	}
	for _, s := range must {
		if !strings.Contains(res.UpContent, s) {
			t.Errorf("missing %q in output:\n%s", s, res.UpContent)
		}
	}
	if filepath.Base(res.UpFile) != "0001_add_orders_table.up.sql" {
		t.Errorf("up file = %q", filepath.Base(res.UpFile))
	}
	if filepath.Base(res.DownFile) != "0001_add_orders_table.down.sql" {
		t.Errorf("down file = %q", filepath.Base(res.DownFile))
	}
	if !strings.HasSuffix(res.UpContent, "\n") {
		t.Errorf("file must end in single newline")
	}
	for _, line := range strings.Split(res.UpContent, "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("trailing whitespace on line: %q", line)
		}
	}
}

func TestCodegen_Postgres_AlterColumn(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 2, Name: "alter_products_price",
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAlterColumn, Table: "products", Column: "price",
				Before: &migrate.ColumnModel{Name: "price", SQLType: "FLOAT"},
				After:  &migrate.ColumnModel{Name: "price", SQLType: "DECIMAL(12,4)"},
				Reason: "type widening",
			},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	res, _ := g.Generate(plan)
	if !strings.Contains(res.UpContent, "ALTER COLUMN price SET DATA TYPE DECIMAL(12,4)") {
		t.Errorf("missing ALTER SET DATA TYPE: %s", res.UpContent)
	}
}

func TestCodegen_Postgres_AddIndex_Unique(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 3, Name: "add_idx",
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAddIndex, Table: "users", Index: "uniq_users_email",
				IndexDef: &migrate.IndexModel{Name: "uniq_users_email", Columns: []string{"email"}, Unique: true},
				Reason:   "uniq",
			},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	res, _ := g.Generate(plan)
	if !strings.Contains(res.UpContent, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS uniq_users_email ON users (email);") {
		t.Errorf("missing unique index DDL: %s", res.UpContent)
	}
}

func TestCodegen_MySQL_AddTable(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "add_orders_table",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "orders", NewTable: makeTable(), Reason: "x"},
		},
	}
	g := codegen.New("mysql", t.TempDir())
	g.Now = fixedNow
	res, _ := g.Generate(plan)
	if !strings.Contains(res.UpContent, "AUTO_INCREMENT") || !strings.Contains(res.UpContent, "ENGINE=InnoDB") {
		t.Errorf("expected MySQL AUTO_INCREMENT + InnoDB: %s", res.UpContent)
	}
}

func TestCodegen_SQLite_AddTable(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "add_orders_table",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "orders", NewTable: makeTable(), Reason: "x"},
		},
	}
	g := codegen.New("sqlite", t.TempDir())
	g.Now = fixedNow
	res, _ := g.Generate(plan)
	if !strings.Contains(res.UpContent, "PRIMARY KEY AUTOINCREMENT") {
		t.Errorf("expected SQLite AUTOINCREMENT: %s", res.UpContent)
	}
}

func TestCodegen_Checksum_Stable(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "x",
		Operations: []migrate.Operation{
			{Kind: migrate.OpDropTable, Table: "x", Reason: "drop"},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	r1, _ := g.Generate(plan)
	plan2 := &migrate.DiffPlan{
		Version: 1, Name: "x",
		Operations: []migrate.Operation{
			{Kind: migrate.OpDropTable, Table: "x", Reason: "drop"},
		},
	}
	r2, _ := g.Generate(plan2)
	if r1.Checksum != r2.Checksum {
		t.Errorf("checksum unstable: %s vs %s", r1.Checksum, r2.Checksum)
	}
}

func TestCodegen_AddColumn_NotNullWithoutDefault_UsesDefaultNull(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "add_age",
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAddColumn, Table: "users", Column: "age",
				After:  &migrate.ColumnModel{Name: "age", SQLType: "INTEGER", Nullable: false},
				Reason: "add age column",
			},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	res, err := g.Generate(plan)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(res.UpContent, "ADD COLUMN IF NOT EXISTS age INTEGER NOT NULL DEFAULT NULL") && !strings.Contains(res.UpContent, "ADD COLUMN age INTEGER NOT NULL DEFAULT NULL") {
		t.Errorf("expected NOT NULL with DEFAULT NULL safety: %q", res.UpContent)
	}
}

func TestCodegen_Postgres_MultipleOpsInOnePlan(t *testing.T) {
	t1 := migrate.NewTableModel("users")
	t1.Columns["id"] = &migrate.ColumnModel{Name: "id", SQLType: "BIGINT", IsPK: true, AutoIncrement: true}
	t1.ColumnOrder = []string{"id"}
	plan := &migrate.DiffPlan{
		Version: 1, Name: "add_users_and_orders",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "users", NewTable: t1, Reason: "add users"},
			{Kind: migrate.OpAddIndex, Table: "users", Index: "idx_users_id",
				IndexDef: &migrate.IndexModel{Name: "idx_users_id", Columns: []string{"id"}}, Reason: "add index"},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	res, err := g.Generate(plan)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.Contains(res.UpContent, "CREATE INDEX CONCURRENTLY") {
		t.Errorf("expected CREATE INDEX CONCURRENTLY in multi-op output: %s", res.UpContent)
	}
}

func TestCodegen_DownHeaderCorrect(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "add_table",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "x", NewTable: migrate.NewTableModel("x"), Reason: "x"},
		},
	}
	g := codegen.New("postgres", t.TempDir())
	g.Now = fixedNow
	res, _ := g.Generate(plan)
	if !strings.Contains(res.DownContent, "-- Direction: DOWN") {
		t.Errorf("down file missing DOWN header: %s", res.DownContent)
	}
}

func TestCodegen_RefusesEmptyPlan(t *testing.T) {
	g := codegen.New("postgres", t.TempDir())
	if _, err := g.Generate(&migrate.DiffPlan{IsEmpty: true}); err == nil {
		t.Fatal("expected error on empty plan")
	}
}

func TestCodegen_MSSQL_AddTable(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "add_orders_table",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "orders", NewTable: makeTable(), Reason: "x"},
		},
	}
	g := codegen.New("mssql", t.TempDir())
	g.Now = fixedNow
	res, _ := g.Generate(plan)
	if !strings.Contains(res.UpContent, "IDENTITY(1,1)") || !strings.Contains(res.UpContent, "CREATE TABLE orders") {
		t.Errorf("expected MSSQL IDENTITY(1,1): %s", res.UpContent)
	}
}
