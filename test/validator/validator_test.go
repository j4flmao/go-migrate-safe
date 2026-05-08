package validator_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/store"
	"github.com/j4flmao/go-migrate-safe/validator"
)

func TestPreGenerate_EmptyPlan(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	plan := &migrate.DiffPlan{IsEmpty: true}
	report := v.PreGenerate(plan)
	if report.Status() != "ok" {
		t.Errorf("Status = %q, want ok", report.Status())
	}
}

func TestPreGenerate_AddTable_Allowed(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "users", Reason: "add"},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "ok" {
		t.Errorf("Status = %q, want ok, errors=%v", report.Status(), report.Errors)
	}
}

func TestPreGenerate_DropTable_Blocked(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowDropTable: false})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpDropTable, Table: "users", IsUnsafe: true, Reason: "drop"},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "error" {
		t.Errorf("Status = %q, want error", report.Status())
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected errors")
	}
}

func TestPreGenerate_DropTable_Allowed(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowDropTable: true})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpDropTable, Table: "users", Reason: "drop"},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "ok" {
		t.Errorf("Status = %q, want ok", report.Status())
	}
}

func TestPreGenerate_DropColumn_Blocked(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowDropColumn: false})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpDropColumn, Table: "users", Column: "legacy", IsUnsafe: true},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "error" {
		t.Errorf("Status = %q, want error", report.Status())
	}
}

func TestPreGenerate_AlterColumn_TypeChange_None(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowTypeChange: migrate.TypeChangeNone})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAlterColumn, Table: "products", Column: "price",
				Before: &migrate.ColumnModel{Name: "price", SQLType: "FLOAT"},
				After:  &migrate.ColumnModel{Name: "price", SQLType: "DECIMAL(12,4)"},
			},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "error" {
		t.Errorf("Status = %q, want error for none", report.Status())
	}
}

func TestPreGenerate_AlterColumn_TypeChange_Safe(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowTypeChange: migrate.TypeChangeSafe})
	// FLOAT -> DECIMAL is lossy
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAlterColumn, Table: "products", Column: "price",
				Before: &migrate.ColumnModel{Name: "price", SQLType: "FLOAT"},
				After:  &migrate.ColumnModel{Name: "price", SQLType: "DECIMAL(12,4)"},
			},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "error" {
		t.Errorf("Status = %q, want error for lossy change", report.Status())
	}
}

func TestPreGenerate_TypeChange_SafeWidening_Allowed(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowTypeChange: migrate.TypeChangeSafe})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAlterColumn, Table: "products", Column: "qty",
				Before: &migrate.ColumnModel{Name: "qty", SQLType: "INTEGER"},
				After:  &migrate.ColumnModel{Name: "qty", SQLType: "BIGINT"},
			},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "ok" {
		t.Errorf("Status = %q, want ok for safe widening, errors=%v", report.Status(), report.Errors)
	}
}

func TestPreGenerate_TypeChange_Lossy_Blocked(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{AllowTypeChange: migrate.TypeChangeSafe})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAlterColumn, Table: "users", Column: "name",
				Before: &migrate.ColumnModel{Name: "name", SQLType: "VARCHAR(100)"},
				After:  &migrate.ColumnModel{Name: "name", SQLType: "VARCHAR(50)"},
			},
		},
	}
	report := v.PreGenerate(plan)
	if report.Status() != "error" {
		t.Errorf("Status = %q, want error for lossy change", report.Status())
	}
}

func TestPreGenerate_AddColumn_WithDefault_Passes(t *testing.T) {
	def := "'active'"
	v := validator.New(migrate.SafetyOptions{})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAddColumn, Table: "users", Column: "status",
				After: &migrate.ColumnModel{Name: "status", SQLType: "TEXT", Nullable: false, Default: &def},
			},
		},
	}
	report := v.PreGenerate(plan)
	if len(report.Errors) != 0 {
		t.Errorf("expected no errors with default, got %v", report.Errors)
	}
}

func TestPreGenerate_WarnNotNullNoDefault(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAddColumn, Table: "users", Column: "age",
				After: &migrate.ColumnModel{Name: "age", SQLType: "INTEGER", Nullable: false},
			},
		},
	}
	report := v.PreGenerate(plan)
	if len(report.Warnings) == 0 {
		t.Error("expected warning for NOT NULL without DEFAULT")
	}
}

func TestFullValidate_VersionGap_Warning(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	files := []store.File{
		{Version: 1, Name: "init", Direction: "up"},
		{Version: 3, Name: "skip", Direction: "up"},
	}
	report := v.FullValidate(context.Background(), files, nil)
	if len(report.Warnings) == 0 {
		t.Error("expected version gap warning")
	}
	warnFound := false
	for _, w := range report.Warnings {
		if w.Code == "WarnVersionGap" {
			warnFound = true
			break
		}
	}
	if !warnFound {
		t.Errorf("expected WarnVersionGap, got %+v", report.Warnings)
	}
}

func TestFullValidate_ChecksumMismatch_Error(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	files := []store.File{
		{Version: 1, Name: "init", Direction: "up", Checksum: "abc123"},
	}
	hist := []driver.MigrationRecord{
		{Version: 1, Direction: "up", Status: "applied", Checksum: "def456"},
	}
	report := v.FullValidate(context.Background(), files, hist)
	if len(report.Errors) == 0 {
		t.Fatal("expected checksum mismatch error")
	}
	if report.Errors[0].Code != "ErrChecksumMismatch" {
		t.Errorf("Code = %q, want ErrChecksumMismatch", report.Errors[0].Code)
	}
}
