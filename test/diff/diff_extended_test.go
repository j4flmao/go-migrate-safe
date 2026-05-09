package diff_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/diff"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestDiff_RenameColumn_Explicit(t *testing.T) {
	want := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("username", "TEXT"),
	))
	db := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("user_name", "TEXT"),
	))
	
	// Use explicit rename spec
	specs := []migrate.RenameSpec{
		{Table: "users", OldName: "user_name", NewName: "username"},
	}
	plan := diff.New(specs...).Compute(want, db, 1)
	
	kinds := opKinds(plan)
	hasRename := false
	for _, k := range kinds {
		if k == migrate.OpRenameColumn {
			hasRename = true
			break
		}
	}
	if !hasRename {
		t.Errorf("expected OpRenameColumn, got %v", kinds)
	}
	
	// Should not have DropColumn or AddColumn for these fields
	for _, op := range plan.Operations {
		if op.Kind == migrate.OpDropColumn && op.Column == "user_name" {
			t.Errorf("spurious drop column")
		}
		if op.Kind == migrate.OpAddColumn && op.Column == "username" {
			t.Errorf("spurious add column")
		}
	}
}

func TestDiff_LossyTypeChange_FlaggedUnsafe(t *testing.T) {
	want := newSchema(newTable("products", newCol("id", "INT")))
	db := newSchema(newTable("products", newCol("id", "BIGINT")))
	
	plan := diff.New().Compute(want, db, 1)
	if !plan.HasDestructiveOps {
		t.Error("BIGINT -> INT should be flagged as destructive (data loss)")
	}
}

func TestDiff_SafeTypeChange_NotUnsafe(t *testing.T) {
	want := newSchema(newTable("products", newCol("id", "BIGINT")))
	db := newSchema(newTable("products", newCol("id", "INT")))
	
	plan := diff.New().Compute(want, db, 1)
	if plan.HasDestructiveOps {
		t.Error("INT -> BIGINT should NOT be flagged as destructive (safe upgrade)")
	}
}

func TestDiff_ConstraintChanges(t *testing.T) {
	wt := newTable("orders", newCol("id", "BIGINT"))
	wt.Constraints["check_price"] = &migrate.ConstraintModel{
		Name: "check_price", Kind: migrate.ConstraintCheck, CheckExpr: "price > 0",
	}
	
	dt := newTable("orders", newCol("id", "BIGINT"))
	dt.Constraints["old_check"] = &migrate.ConstraintModel{
		Name: "old_check", Kind: migrate.ConstraintCheck, CheckExpr: "price >= 0",
	}
	
	plan := diff.New().Compute(newSchema(wt), newSchema(dt), 1)
	kinds := opKinds(plan)
	
	hasDrop := false
	hasAdd := false
	for _, k := range kinds {
		if k == migrate.OpDropConstraint { hasDrop = true }
		if k == migrate.OpAddConstraint { hasAdd = true }
	}
	if !hasDrop || !hasAdd {
		t.Errorf("expected drop + add constraints, got %v", kinds)
	}
}
