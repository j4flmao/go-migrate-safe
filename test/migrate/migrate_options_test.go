package migrate_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestMigrate_Options(t *testing.T) {
	m, err := migrate.New(
		migrate.WithDriver("mysql"),
		migrate.WithSchema("test_db"),
		migrate.WithOutputDir("./custom_migrations"),
		migrate.WithAllowDropTable(),
		migrate.WithAllowDropColumn(),
		migrate.WithAllowTypeChange(migrate.TypeChangeSafe),
		migrate.WithNoRollbackRequired(),
		migrate.WithAutoBackfillStep(),
		migrate.WithRenameColumn("users", "old_name", "new_name"),
		migrate.WithVersionStyle("timestamp"),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if m.Driver() != "mysql" {
		t.Errorf("expected mysql, got %s", m.Driver())
	}
	if m.Schema() != "test_db" {
		t.Errorf("expected test_db, got %s", m.Schema())
	}
	if m.OutputDir() != "./custom_migrations" {
		t.Errorf("expected ./custom_migrations, got %s", m.OutputDir())
	}

	opts := m.SafetyOptions()
	if !opts.AllowDropTable {
		t.Error("AllowDropTable should be true")
	}
	if !opts.AllowDropColumn {
		t.Error("AllowDropColumn should be true")
	}
	if opts.AllowTypeChange != migrate.TypeChangeSafe {
		t.Errorf("expected TypeChangeSafe, got %v", opts.AllowTypeChange)
	}
	if !opts.NoRollbackRequired {
		t.Error("NoRollbackRequired should be true")
	}
	if !opts.AutoBackfillStep {
		t.Error("AutoBackfillStep should be true")
	}

	renames := m.RenameSpecs()
	if len(renames) != 1 || renames[0].Table != "users" || renames[0].OldName != "old_name" || renames[0].NewName != "new_name" {
		t.Errorf("RenameSpecs wrong: %+v", renames)
	}
}

func TestMigrate_DefaultOptions(t *testing.T) {
	m, _ := migrate.New()
	if m.Driver() != "postgres" {
		t.Errorf("default driver should be postgres, got %s", m.Driver())
	}
	if m.Schema() != "public" {
		t.Errorf("default schema should be public, got %s", m.Schema())
	}

	opts := m.SafetyOptions()
	if opts.AllowDropTable || opts.AllowDropColumn || opts.AllowTypeChange != migrate.TypeChangeNone {
		t.Error("default safety options should be restrictive")
	}
}
