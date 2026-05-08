package diff_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/diff"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func newCol(name, typ string, opts ...func(*migrate.ColumnModel)) *migrate.ColumnModel {
	c := &migrate.ColumnModel{Name: name, SQLType: typ}
	for _, o := range opts {
		o(c)
	}
	return c
}

func newTable(name string, cols ...*migrate.ColumnModel) *migrate.TableModel {
	t := migrate.NewTableModel(name)
	for _, c := range cols {
		t.Columns[c.Name] = c
		t.ColumnOrder = append(t.ColumnOrder, c.Name)
	}
	return t
}

func newSchema(tables ...*migrate.TableModel) *migrate.SchemaModel {
	sm := migrate.NewSchemaModel("public")
	for _, t := range tables {
		sm.Tables[t.Name] = t
	}
	return sm
}

func opKinds(p *migrate.DiffPlan) []migrate.OpKind {
	out := make([]migrate.OpKind, 0, len(p.Operations))
	for _, op := range p.Operations {
		out = append(out, op.Kind)
	}
	return out
}

func TestDiff_AddTable(t *testing.T) {
	want := newSchema(newTable("orders", newCol("id", "BIGINT")))
	db := newSchema()
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAddTable {
		t.Errorf("ops = %v, want [add_table]", got)
	}
	if plan.HasDestructiveOps {
		t.Errorf("add_table should not be destructive")
	}
}

func TestDiff_DropTable_Unsafe(t *testing.T) {
	want := newSchema()
	db := newSchema(newTable("orders", newCol("id", "BIGINT")))
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpDropTable {
		t.Errorf("ops = %v, want [drop_table]", got)
	}
	if !plan.HasDestructiveOps {
		t.Errorf("drop_table should be destructive")
	}
}

func TestDiff_AddColumn(t *testing.T) {
	want := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("email", "TEXT"),
	))
	db := newSchema(newTable("users", newCol("id", "BIGINT")))
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAddColumn {
		t.Errorf("ops = %v", got)
	}
}

func TestDiff_DropColumn_Unsafe(t *testing.T) {
	want := newSchema(newTable("users", newCol("id", "BIGINT")))
	db := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("legacy", "TEXT"),
	))
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpDropColumn {
		t.Errorf("ops = %v", got)
	}
	if !plan.HasDestructiveOps {
		t.Errorf("drop_column should be destructive")
	}
}

func TestDiff_AlterColumnType(t *testing.T) {
	want := newSchema(newTable("products",
		newCol("price", "DECIMAL(12,4)"),
	))
	db := newSchema(newTable("products",
		newCol("price", "FLOAT"),
	))
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAlterColumn {
		t.Errorf("ops = %v", got)
	}
}

func TestDiff_AlterColumnNullability(t *testing.T) {
	want := newSchema(newTable("users",
		newCol("name", "TEXT", func(c *migrate.ColumnModel) { c.Nullable = true }),
	))
	db := newSchema(newTable("users",
		newCol("name", "TEXT"),
	))
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAlterColumn {
		t.Errorf("ops = %v", got)
	}
}

func TestDiff_AddIndex(t *testing.T) {
	wt := newTable("users", newCol("email", "TEXT"))
	wt.Indexes["idx_users_email"] = &migrate.IndexModel{Name: "idx_users_email", Columns: []string{"email"}}
	dt := newTable("users", newCol("email", "TEXT"))
	plan := diff.New().Compute(newSchema(wt), newSchema(dt), 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAddIndex {
		t.Errorf("ops = %v", got)
	}
}

func TestDiff_DropIndex(t *testing.T) {
	wt := newTable("users", newCol("email", "TEXT"))
	dt := newTable("users", newCol("email", "TEXT"))
	dt.Indexes["idx_users_email"] = &migrate.IndexModel{Name: "idx_users_email", Columns: []string{"email"}}
	plan := diff.New().Compute(newSchema(wt), newSchema(dt), 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpDropIndex {
		t.Errorf("ops = %v", got)
	}
}

func TestDiff_AddFK(t *testing.T) {
	wt := newTable("orders", newCol("user_id", "BIGINT"))
	wt.Constraints["fk_orders_users"] = &migrate.ConstraintModel{
		Name: "fk_orders_users", Kind: migrate.ConstraintForeignKey,
		Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"},
	}
	dt := newTable("orders", newCol("user_id", "BIGINT"))
	plan := diff.New().Compute(newSchema(wt), newSchema(dt), 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAddConstraint {
		t.Errorf("ops = %v", got)
	}
}

func TestDiff_OperationOrder_DropFKBeforeDropTable(t *testing.T) {
	dt1 := newTable("orders", newCol("user_id", "BIGINT"))
	dt1.Constraints["fk_orders_users"] = &migrate.ConstraintModel{
		Name: "fk_orders_users", Kind: migrate.ConstraintForeignKey,
		Columns: []string{"user_id"}, RefTable: "users", RefColumns: []string{"id"},
	}
	dt2 := newTable("users", newCol("id", "BIGINT"))
	want := newSchema(newTable("orders", newCol("user_id", "BIGINT")))
	db := newSchema(dt1, dt2)
	plan := diff.New().Compute(want, db, 1)
	kinds := opKinds(plan)
	// Expect: drop_constraint (FK), drop_table users
	dropFKIdx, dropTblIdx := -1, -1
	for i, k := range kinds {
		if k == migrate.OpDropConstraint && dropFKIdx == -1 {
			dropFKIdx = i
		}
		if k == migrate.OpDropTable && dropTblIdx == -1 {
			dropTblIdx = i
		}
	}
	if dropFKIdx < 0 || dropTblIdx < 0 || dropFKIdx > dropTblIdx {
		t.Errorf("FK drop must come before table drop. ops=%v", kinds)
	}
}

func TestDiff_EmptyDiff(t *testing.T) {
	tbl := newTable("users", newCol("id", "BIGINT"))
	plan := diff.New().Compute(newSchema(tbl), newSchema(tbl), 1)
	if !plan.IsEmpty {
		t.Errorf("expected empty plan, got %d ops", len(plan.Operations))
	}
}

func TestDiff_RenameHint(t *testing.T) {
	want := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("username", "TEXT"),
	))
	db := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("user_name", "TEXT"),
	))
	plan := diff.New().Compute(want, db, 1)
	if len(plan.RenameHints) == 0 {
		t.Fatalf("expected rename hint, got 0")
	}
	h := plan.RenameHints[0]
	if h.DroppedColumn != "user_name" || h.AddedColumn != "username" {
		t.Errorf("unexpected hint: %+v", h)
	}
	if h.Confidence != "high" {
		t.Errorf("expected high confidence rename hint, got %q", h.Confidence)
	}
}

func TestDiff_AlterColumnDefaultChanged(t *testing.T) {
	def1 := "10"
	def2 := "20"
	want := newSchema(newTable("users",
		newCol("score", "INTEGER", func(c *migrate.ColumnModel) { c.Default = &def2 }),
	))
	db := newSchema(newTable("users",
		newCol("score", "INTEGER", func(c *migrate.ColumnModel) { c.Default = &def1 }),
	))
	plan := diff.New().Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpAlterColumn {
		t.Errorf("ops = %v, want [alter_column]", got)
	}
}

func TestDiff_OperationOrder_AddTableBeforeAddColumn(t *testing.T) {
	wt := newSchema(
		newTable("users", newCol("id", "BIGINT"), newCol("email", "TEXT")),
		newTable("orders", newCol("id", "BIGINT"), newCol("user_id", "BIGINT")),
	)
	dt := newSchema(newTable("users", newCol("id", "BIGINT")))
	plan := diff.New().Compute(wt, dt, 1)
	kinds := opKinds(plan)
	addTblIdx, addColIdx := -1, -1
	for i, k := range kinds {
		if k == migrate.OpAddTable && addTblIdx == -1 {
			addTblIdx = i
		}
		if k == migrate.OpAddColumn && addColIdx == -1 {
			addColIdx = i
		}
	}
	if addTblIdx < 0 || addColIdx < 0 || addTblIdx > addColIdx {
		t.Errorf("AddTable must come before AddColumn. ops=%v", kinds)
	}
}

func TestDiff_HasDestructiveOps_WhenDropPresent(t *testing.T) {
	want := newSchema()
	db := newSchema(newTable("users", newCol("id", "BIGINT")))
	plan := diff.New().Compute(want, db, 1)
	if !plan.HasDestructiveOps {
		t.Error("HasDestructiveOps should be true when drop present")
	}
}

func TestDiff_NoDestructiveOps_ForAddOnly(t *testing.T) {
	want := newSchema(newTable("users", newCol("id", "BIGINT")))
	db := newSchema()
	plan := diff.New().Compute(want, db, 1)
	if plan.HasDestructiveOps {
		t.Error("HasDestructiveOps should be false for add-only")
	}
}

func TestDiff_ExplicitRename(t *testing.T) {
	want := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("username", "TEXT"),
	))
	db := newSchema(newTable("users",
		newCol("id", "BIGINT"),
		newCol("user_name", "TEXT"),
	))
	plan := diff.New(migrate.RenameSpec{Table: "users", OldName: "user_name", NewName: "username"}).
		Compute(want, db, 1)
	if got := opKinds(plan); len(got) != 1 || got[0] != migrate.OpRenameColumn {
		t.Errorf("expected single rename op, got %v", got)
	}
}
