package codegen_test

import (
	"strings"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/codegen"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestCodegen_Dialects(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1,
		Name:    "init",
		Operations: []migrate.Operation{
			{
				Kind:  migrate.OpAddTable,
				Table: "users",
				NewTable: &migrate.TableModel{
					Name: "users",
					Columns: map[string]*migrate.ColumnModel{
						"id":   {Name: "id", SQLType: "BIGINT", IsPK: true},
						"name": {Name: "name", SQLType: "VARCHAR(255)", Nullable: false},
					},
					ColumnOrder: []string{"id", "name"},
				},
			},
		},
	}

	dialects := []string{"postgres", "mysql", "sqlite", "mssql"}
	for _, d := range dialects {
		t.Run(d, func(t *testing.T) {
			tmpDir := t.TempDir()
			g := codegen.New(d, tmpDir)
			g.Now = func() time.Time { return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) }

			res, err := g.Generate(plan)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}

			if res.UpContent == "" {
				t.Error("UpContent is empty")
			}

			// Check for dialect-specific syntax if any
			switch d {
			case "mysql":
				if !strings.Contains(res.UpContent, "CREATE TABLE IF NOT EXISTS users") {
					t.Errorf("MySQL output missing table name: %s", res.UpContent)
				}
			case "postgres":
				if !strings.Contains(res.UpContent, "CREATE TABLE IF NOT EXISTS users") {
					t.Errorf("Postgres output missing table name: %s", res.UpContent)
				}
			case "mssql":
				if !strings.Contains(res.UpContent, "CREATE TABLE users") {
					t.Errorf("MSSQL output missing table name: %s", res.UpContent)
				}
			}
		})
	}
}

func TestCodegen_MongoDB(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1,
		Name:    "init_mongo",
		Operations: []migrate.Operation{
			{
				Kind:  migrate.OpAddTable,
				Table: "users",
			},
		},
	}

	tmpDir := t.TempDir()
	g := codegen.New("mongodb", tmpDir)
	res, err := g.Generate(plan)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.HasSuffix(res.UpFile, ".jsonc") {
		t.Errorf("expected .jsonc for mongodb, got %s", res.UpFile)
	}
	if !strings.Contains(res.UpContent, "\"create\": \"users\"") {
		t.Errorf("MongoDB output missing create command: %s", res.UpContent)
	}
}
