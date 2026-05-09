package rollback_test

import (
	"strings"
	"testing"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/rollback"
)

func TestRollback_AddTable_InvertsToDropTable(t *testing.T) {
	plan := &migrate.DiffPlan{
		Version: 1, Name: "x",
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "users",
				NewTable: migrate.NewTableModel("users")},
		},
	}
	rp, err := rollback.New("postgres").Build(plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rp.RollbackOps) != 1 || rp.RollbackOps[0].Kind != migrate.OpDropTable {
		t.Fatalf("ops = %+v", rp.RollbackOps)
	}
	if rp.RequiresManual {
		t.Errorf("add_table inverse must be safe")
	}
}

func TestRollback_DropTable_RequiresManual(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpDropTable, Table: "old", IsUnsafe: true},
		},
	}
	rp, err := rollback.New("postgres").Build(plan, "")
	if err != nil {
		t.Fatal(err)
	}
	if !rp.RequiresManual {
		t.Errorf("drop_table inverse must require manual")
	}
	if !strings.Contains(rp.RollbackOps[0].SQL, "REQUIRES MANUAL") {
		t.Errorf("stub missing manual marker: %s", rp.RollbackOps[0].SQL)
	}
}

func TestRollback_AddColumn_InvertsToDropColumn(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddColumn, Table: "users", Column: "age",
				After: &migrate.ColumnModel{Name: "age", SQLType: "INTEGER"}},
		},
	}
	rp, _ := rollback.New("postgres").Build(plan, "")
	if rp.RollbackOps[0].Kind != migrate.OpDropColumn {
		t.Fatalf("ops = %+v", rp.RollbackOps)
	}
}

func TestRollback_DropColumn_RequiresManual(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpDropColumn, Table: "users", Column: "legacy",
				Before:   &migrate.ColumnModel{Name: "legacy", SQLType: "BIGINT"},
				IsUnsafe: true,
			},
		},
	}
	rp, _ := rollback.New("postgres").Build(plan, "")
	if !rp.RequiresManual {
		t.Errorf("drop_column inverse must require manual")
	}
	if !strings.Contains(rp.RollbackOps[0].SQL, "BIGINT") {
		t.Errorf("stub should reference original type: %s", rp.RollbackOps[0].SQL)
	}
}

func TestRollback_AlterColumn_ReverseTypeChange(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind: migrate.OpAlterColumn, Table: "products", Column: "price",
				Before: &migrate.ColumnModel{Name: "price", SQLType: "FLOAT"},
				After:  &migrate.ColumnModel{Name: "price", SQLType: "DECIMAL(12,4)"},
			},
		},
	}
	rp, _ := rollback.New("postgres").Build(plan, "")
	if !strings.Contains(rp.RollbackOps[0].SQL, "SET DATA TYPE FLOAT") {
		t.Errorf("expected reverse type: %s", rp.RollbackOps[0].SQL)
	}
}

func TestRollback_Order_IsReverseOfForward(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{Kind: migrate.OpAddTable, Table: "a", NewTable: migrate.NewTableModel("a")},
			{Kind: migrate.OpAddColumn, Table: "a", Column: "x",
				After: &migrate.ColumnModel{Name: "x", SQLType: "INTEGER"}},
		},
	}
	rp, _ := rollback.New("postgres").Build(plan, "")
	if rp.RollbackOps[0].Column != "x" {
		t.Errorf("expected last forward op (add_column) to be first rollback op, got %+v", rp.RollbackOps)
	}
}

func TestRollback_RenameColumn_InvertsToRenameBack(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind:   migrate.OpRenameColumn,
				Table:  "users",
				Before: &migrate.ColumnModel{Name: "old_name", SQLType: "TEXT"},
				After:  &migrate.ColumnModel{Name: "new_name", SQLType: "TEXT"},
			},
		},
	}

	rp, _ := rollback.New("postgres").Build(plan, "")
	rb := rp.RollbackOps[0]
	if rb.Kind != migrate.OpRenameColumn {
		t.Errorf("expected OpRenameColumn, got %v", rb.Kind)
	}
	if rb.Before.Name != "new_name" || rb.After.Name != "old_name" {
		t.Errorf("rename inversion failed: %s -> %s", rb.Before.Name, rb.After.Name)
	}
}

func TestRollback_AlterColumn_Nullability(t *testing.T) {
	plan := &migrate.DiffPlan{
		Operations: []migrate.Operation{
			{
				Kind:   migrate.OpAlterColumn,
				Table:  "users",
				Before: &migrate.ColumnModel{Name: "email", Nullable: true},
				After:  &migrate.ColumnModel{Name: "email", Nullable: false},
			},
		},
	}

	rp, _ := rollback.New("postgres").Build(plan, "")
	rb := rp.RollbackOps[0]
	if rb.Kind != migrate.OpAlterColumn {
		t.Errorf("expected OpAlterColumn, got %v", rb.Kind)
	}
	if rb.After.Nullable != true {
		t.Error("should invert NOT NULL back to NULLABLE")
	}
}
