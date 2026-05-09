package validator_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/store"
	"github.com/j4flmao/go-migrate-safe/validator"
)

func TestValidator_FullValidate_ChecksumDrift(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	files := []store.File{
		{Version: 1, Name: "init", Direction: "up", Checksum: "modified_hash"},
	}
	hist := []driver.MigrationRecord{
		{Version: 1, Name: "init", Direction: "up", Checksum: "original_hash", Status: "applied"},
	}

	report := v.FullValidate(context.Background(), files, hist)
	if report.Status() != "error" {
		t.Errorf("expected error status for checksum mismatch, got %s", report.Status())
	}

	found := false
	for _, err := range report.Errors {
		if err.Code == "ErrChecksumMismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing ErrChecksumMismatch in report")
	}
}

func TestValidator_FullValidate_VersionGap(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	files := []store.File{
		{Version: 1, Name: "v1", Direction: "up"},
		{Version: 3, Name: "v3", Direction: "up"}, // Missing v2
	}

	report := v.FullValidate(context.Background(), files, nil)
	if report.Status() != "warning" {
		t.Errorf("expected warning status for version gap, got %s", report.Status())
	}

	found := false
	for _, warn := range report.Warnings {
		if warn.Code == "WarnVersionGap" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing WarnVersionGap in report")
	}
}

func TestValidator_PreGenerate_NotNullWithNoDefault(t *testing.T) {
	v := validator.New(migrate.SafetyOptions{})
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind:  migrate.OpAddColumn,
				Table: "users",
				Column: "email",
				After: &migrate.ColumnModel{Name: "email", SQLType: "TEXT", Nullable: false, Default: nil},
			},
		},
	}

	report := v.PreGenerate(plan)
	if report.Status() != "warning" {
		t.Errorf("expected warning for NOT NULL column without default, got %s", report.Status())
	}

	found := false
	for _, warn := range report.Warnings {
		if warn.Code == "WarnAddNotNullWithoutDefault" {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing WarnAddNotNullWithoutDefault in report")
	}
}
