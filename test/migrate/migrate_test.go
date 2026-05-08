package migrate_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/memory"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestNewMigrator_Defaults(t *testing.T) {
	m, err := migrate.New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if m.OutputDir() != "./migrations" {
		t.Errorf("OutputDir = %q", m.OutputDir())
	}
	if m.Schema() != "public" {
		t.Errorf("Schema = %q", m.Schema())
	}
	if m.Driver() != "postgres" {
		t.Errorf("Driver = %q", m.Driver())
	}
}

func TestWithModels(t *testing.T) {
	type User struct {
		ID   int64 `db:"id,pk,autoincrement"`
		Name string `db:"name"`
	}
	m, err := migrate.New(migrate.WithModels(User{}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	models := m.Models()
	if len(models) != 1 {
		t.Fatalf("Models len = %d", len(models))
	}
}

func TestWithOutputDir(t *testing.T) {
	m, _ := migrate.New(migrate.WithOutputDir("/tmp/migrations"))
	if m.OutputDir() != "/tmp/migrations" {
		t.Errorf("OutputDir = %q", m.OutputDir())
	}
}

func TestWithSchema(t *testing.T) {
	m, _ := migrate.New(migrate.WithSchema("app"))
	if m.Schema() != "app" {
		t.Errorf("Schema = %q", m.Schema())
	}
}

func TestWithDriver(t *testing.T) {
	m, _ := migrate.New(migrate.WithDriver("mysql"))
	if m.Driver() != "mysql" {
		t.Errorf("Driver = %q", m.Driver())
	}
}

func TestSafetyOptions(t *testing.T) {
	m, _ := migrate.New(
		migrate.WithAllowDropTable(),
		migrate.WithAllowDropColumn(),
		migrate.WithAllowTypeChange(migrate.TypeChangeAny),
	)
	opts := m.SafetyOptions()
	if !opts.AllowDropTable {
		t.Error("AllowDropTable = false, want true")
	}
	if !opts.AllowDropColumn {
		t.Error("AllowDropColumn = false, want true")
	}
	if opts.AllowTypeChange != migrate.TypeChangeAny {
		t.Errorf("AllowTypeChange = %v, want any", opts.AllowTypeChange)
	}
}

func TestWithRenameColumn(t *testing.T) {
	m, _ := migrate.New(migrate.WithRenameColumn("users", "user_name", "username"))
	specs := m.RenameSpecs()
	if len(specs) != 1 {
		t.Fatalf("RenameSpecs len = %d", len(specs))
	}
	if specs[0].Table != "users" || specs[0].OldName != "user_name" || specs[0].NewName != "username" {
		t.Errorf("RenameSpec = %+v", specs[0])
	}
}

func TestContext(t *testing.T) {
	m, _ := migrate.New(migrate.WithDriver("sqlite"), migrate.WithOutputDir("/migrations"))
	ctx := m.Context(5, []int64{1, 2, 3})
	if ctx.DBDriver != "sqlite" {
		t.Errorf("DBDriver = %q", ctx.DBDriver)
	}
	if ctx.OutputDir != "/migrations" {
		t.Errorf("OutputDir = %q", ctx.OutputDir)
	}
	if ctx.NextVersion != 5 {
		t.Errorf("NextVersion = %d", ctx.NextVersion)
	}
}

func TestDiffPlan_Explain_Empty(t *testing.T) {
	p := &migrate.DiffPlan{IsEmpty: true}
	got := p.Explain()
	if got != "No schema changes detected. Nothing to generate.\n" {
		t.Errorf("Explain = %q", got)
	}
}

func TestDiffPlan_Explain_WithOps(t *testing.T) {
	p := &migrate.DiffPlan{
		Version: 1, Name: "add_users_table",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "users", Reason: "Added users table"},
		},
	}
	got := p.Explain()
	if got == "" {
		t.Fatal("Explain returned empty")
	}
}

func TestDiffPlan_Generate_Stub(t *testing.T) {
	p := &migrate.DiffPlan{Version: 1, Name: "test"}
	_, err := p.Generate()
	if err == nil {
		t.Fatal("expected error (stub)")
	}
}

func TestDiffPlan_Apply_Stub(t *testing.T) {
	p := &migrate.DiffPlan{Version: 1}
	err := p.Apply(nil)
	if err == nil {
		t.Fatal("expected error (stub)")
	}
}

func TestDiffPlan_DryRun_Stub(t *testing.T) {
	p := &migrate.DiffPlan{Version: 1}
	_, err := p.DryRun(nil)
	if err == nil {
		t.Fatal("expected error (stub)")
	}
}

func TestHasDB_WithoutDriver(t *testing.T) {
	m, _ := migrate.New()
	if m.HasDB() {
		t.Error("HasDB = true, want false (no driver)")
	}
}

func TestHasDB_WithDriverInstance(t *testing.T) {
	m, _ := migrate.New(migrate.WithDriverInstance(memory.New()))
	if !m.HasDB() {
		t.Error("HasDB = false, want true (with driver)")
	}
}

func TestHistory_WithoutDriver_ReturnsError(t *testing.T) {
	m, _ := migrate.New()
	_, err := m.History(nil)
	if err == nil {
		t.Fatal("expected error without driver")
	}
}

func TestDB_ReturnsNilByDefault(t *testing.T) {
	m, _ := migrate.New()
	if m.DB() != nil {
		t.Error("DB() should be nil by default")
	}
}

func TestDriverInstance_ReturnsNilByDefault(t *testing.T) {
	m, _ := migrate.New()
	if m.DriverInstance() != nil {
		t.Error("DriverInstance() should be nil by default")
	}
}

func TestWithDriverInstance(t *testing.T) {
	md := memory.New()
	m, _ := migrate.New(migrate.WithDriverInstance(md))
	if m.DriverInstance() != md {
		t.Error("DriverInstance mismatch")
	}
	if !m.HasDB() {
		t.Error("HasDB should be true")
	}
}

func TestMigrator_Status_WithoutDriver_ReturnsError(t *testing.T) {
	m, _ := migrate.New()
	_, err := m.Status(nil)
	if err == nil {
		t.Fatal("expected error without driver")
	}
}

func TestMigrator_Status_WithDriver(t *testing.T) {
	md := memory.New()
	m, _ := migrate.New(migrate.WithDriverInstance(md))
	report, err := m.Status(nil)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report == nil {
		t.Fatal("Status returned nil")
	}
}

func TestMigrator_History_WithDriver(t *testing.T) {
	md := memory.New()
	m, _ := migrate.New(migrate.WithDriverInstance(md))
	hist, err := m.History(nil)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 0 {
		t.Errorf("History len = %d, want 0", len(hist))
	}
}

func TestMigrator_Validate_WithoutDriver_ReturnsError(t *testing.T) {
	m, _ := migrate.New()
	_, err := m.Validate(nil)
	if err == nil {
		t.Fatal("expected error without driver")
	}
}

func TestMigrator_Apply_WithoutDriver_ReturnsError(t *testing.T) {
	m, _ := migrate.New()
	err := m.Apply(nil)
	if err == nil {
		t.Fatal("expected error without driver")
	}
}

func TestMigrator_Rollback_WithoutDriver_ReturnsError(t *testing.T) {
	m, _ := migrate.New()
	err := m.Rollback(nil)
	if err == nil {
		t.Fatal("expected error without driver")
	}
}
