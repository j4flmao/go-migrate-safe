// Package rollback produces compensating SQL/JSON for every forward Operation
// in a DiffPlan, completing the .down.sql or .down.json file that codegen stubbed.
package rollback

import (
	"fmt"
	"os"
	"strings"

	"github.com/j4flmao/go-migrate-safe/codegen"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

// Planner builds rollback SQL per dialect.
type Planner struct {
	Dialect string
}

// New constructs a Planner.
func New(dialect string) *Planner { return &Planner{Dialect: dialect} }

// Plan computes the inverse of plan.Operations and stores both the operations
// and a complete .down.sql body. ManualDownItems lists any rollbacks that
// could not be auto-generated.
type Plan struct {
	ForwardVersion  int64
	RollbackOps     []migrate.Operation
	RequiresManual  bool
	ManualDownItems []ManualItem
	DownContent     string
}

// ManualItem describes a rollback the user must complete by hand.
type ManualItem struct {
	Table  string
	Reason string
	Stub   string
}

func (p *Planner) commentPrefix() string {
	if p.Dialect == "mongodb" {
		return "//"
	}
	return "--"
}

// Build computes the rollback plan and rewrites the .down.sql/.down.json file at downPath.
// Pass downPath = "" to skip filesystem writes (RenderOnly mode).
func (p *Planner) Build(plan *migrate.DiffPlan, downPath string) (*Plan, error) {
	out := &Plan{ForwardVersion: plan.Version}
	// Reverse iteration.
	for i := len(plan.Operations) - 1; i >= 0; i-- {
		op := plan.Operations[i]
		ro, manual, err := p.invert(op)
		if err != nil {
			return nil, err
		}
		out.RollbackOps = append(out.RollbackOps, ro)
		if manual != nil {
			out.RequiresManual = true
			out.ManualDownItems = append(out.ManualDownItems, *manual)
		}
	}
	plan.RollbackOps = out.RollbackOps

	// Render the .down.sql/.down.json body.
	body := p.renderDownBody(plan, out.RollbackOps)
	out.DownContent = body

	if downPath != "" {
		// Replace just the body of the existing down file (preserve the header
		// codegen wrote). We splice after the first blank line that follows
		// header comment lines.
		raw, err := os.ReadFile(downPath)
		if err != nil {
			return nil, fmt.Errorf("rollback: read %s: %w", downPath, err)
		}
		newContent := p.replaceBody(string(raw), body)
		if err := os.WriteFile(downPath, []byte(newContent), 0o644); err != nil {
			return nil, fmt.Errorf("rollback: write %s: %w", downPath, err)
		}
	}
	return out, nil
}

func (p *Planner) invert(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	switch op.Kind {
	case migrate.OpAddTable:
		return p.invertAddTable(op)
	case migrate.OpDropTable:
		return p.invertDropTable(op)
	case migrate.OpAddColumn:
		return p.invertAddColumn(op)
	case migrate.OpDropColumn:
		return p.invertDropColumn(op)
	case migrate.OpAlterColumn:
		return p.invertAlterColumn(op)
	case migrate.OpRenameColumn:
		return p.invertRenameColumn(op)
	case migrate.OpAddIndex:
		return p.invertAddIndex(op)
	case migrate.OpDropIndex:
		return p.invertDropIndex(op)
	case migrate.OpAddConstraint:
		return p.invertAddConstraint(op)
	case migrate.OpDropConstraint:
		return p.invertDropConstraint(op)
	}
	return migrate.Operation{}, nil, fmt.Errorf("rollback: unsupported op %s", op.Kind)
}

func (p *Planner) invertAddTable(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	var sql string
	if p.Dialect == "mongodb" {
		sql = fmt.Sprintf(`{"drop": %q}`, op.Table)
	} else {
		sql = fmt.Sprintf("DROP TABLE IF EXISTS %s;", op.Table)
	}
	return migrate.Operation{
		Kind: migrate.OpDropTable, Table: op.Table,
		SQL:    sql,
		Reason: fmt.Sprintf("Inverse of add_table %s", op.Table),
	}, nil, nil
}

func (p *Planner) invertDropTable(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	c := p.commentPrefix()
	var stub string
	if p.Dialect == "mongodb" {
		stub = fmt.Sprintf(
			"%s REQUIRES MANUAL COMPLETION\n"+
				"%s Forward migration dropped collection %q. Data cannot be recovered automatically.\n"+
				"%s To restore: restore from backup or recreate manually.", c, c, op.Table, c)
	} else {
		stub = fmt.Sprintf(
			"%s REQUIRES MANUAL COMPLETION\n"+
				"%s Forward migration dropped table %q. Data cannot be recovered automatically.\n"+
				"%s To complete:\n"+
				"%s   Option A: restore from backup.\n"+
				"%s   Option B: uncomment and fill in:\n"+
				"%s CREATE TABLE %s (\n"+
				"%s     -- TODO: original columns here\n"+
				"%s );", c, c, op.Table, c, c, c, c, op.Table, c, c)
	}
	ro := migrate.Operation{
		Kind: migrate.OpAddTable, Table: op.Table,
		SQL:    stub,
		Reason: "Manual rollback required for dropped table",
	}
	return ro, &ManualItem{Table: op.Table, Reason: "drop_table cannot be auto-rolled-back", Stub: stub}, nil
}

func (p *Planner) invertAddColumn(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	if p.Dialect == "mongodb" {
		return migrate.Operation{
			Kind:   migrate.OpDropColumn,
			Table:  op.Table,
			Column: op.Column,
			SQL:    "",
			Reason: fmt.Sprintf("Inverse of add_column %s.%s (no-op for mongodb)", op.Table, op.Column),
		}, nil, nil
	}
	return migrate.Operation{
		Kind: migrate.OpDropColumn, Table: op.Table, Column: op.Column,
		SQL:    fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", op.Table, op.Column),
		Reason: fmt.Sprintf("Inverse of add_column %s.%s", op.Table, op.Column),
	}, nil, nil
}

func (p *Planner) invertDropColumn(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	if p.Dialect == "mongodb" {
		return migrate.Operation{
			Kind:   migrate.OpAddColumn,
			Table:  op.Table,
			Column: op.Column,
			SQL:    "",
			Reason: fmt.Sprintf("Inverse of drop_column %s.%s (no-op for mongodb)", op.Table, op.Column),
		}, nil, nil
	}
	typ := "<UNKNOWN_TYPE>"
	if op.Before != nil {
		typ = op.Before.SQLType
	}
	c := p.commentPrefix()
	stub := fmt.Sprintf(
		"%s REQUIRES MANUAL COMPLETION\n"+
			"%s Forward migration dropped %s.%s (%s). Data cannot be recovered automatically.\n"+
			"%s To restore the column without data:\n"+
			"%s ALTER TABLE %s ADD COLUMN %s %s;",
		c, c, op.Table, op.Column, typ, c, c, op.Table, op.Column, typ)
	ro := migrate.Operation{
		Kind: migrate.OpAddColumn, Table: op.Table, Column: op.Column,
		SQL:    stub,
		Reason: "Manual rollback required for dropped column",
	}
	return ro, &ManualItem{Table: op.Table, Reason: "drop_column cannot be auto-rolled-back", Stub: stub}, nil
}

func (p *Planner) invertAlterColumn(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	if p.Dialect == "mongodb" {
		return migrate.Operation{
			Kind:   migrate.OpAlterColumn,
			Table:  op.Table,
			Column: op.Column,
			SQL:    "",
			Reason: fmt.Sprintf("Inverse of alter_column %s.%s (no-op for mongodb)", op.Table, op.Column),
		}, nil, nil
	}
	rev := migrate.Operation{
		Kind: migrate.OpAlterColumn, Table: op.Table, Column: op.Column,
		Before: op.After, After: op.Before,
		Reason: fmt.Sprintf("Inverse of alter_column %s.%s", op.Table, op.Column),
	}
	sql, err := codegen.RenderPostgres(&rev)
	if p.Dialect == "mysql" {
		sql, err = codegen.RenderMySQL(&rev)
	}
	if p.Dialect == "sqlite" {
		sql, err = codegen.RenderSQLite(&rev)
	}
	if err != nil {
		return rev, nil, err
	}
	rev.SQL = sql
	return rev, nil, nil
}

func (p *Planner) invertRenameColumn(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	if p.Dialect == "mongodb" {
		return migrate.Operation{
			Kind:   migrate.OpRenameColumn,
			Table:  op.Table,
			Column: op.Column,
			SQL:    "",
			Reason: fmt.Sprintf("Inverse of rename_column %s.%s (no-op for mongodb)", op.Table, op.Column),
		}, nil, nil
	}
	if op.Before == nil || op.After == nil {
		return migrate.Operation{}, nil, fmt.Errorf("rename inverse needs before/after")
	}
	rev := migrate.Operation{
		Kind: migrate.OpRenameColumn, Table: op.Table,
		Column: op.Before.Name,
		Before: op.After, After: op.Before,
		SQL:    fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", op.Table, op.After.Name, op.Before.Name),
		Reason: fmt.Sprintf("Inverse of rename_column %s.%s -> %s", op.Table, op.Before.Name, op.After.Name),
	}
	return rev, nil, nil
}

func (p *Planner) invertAddIndex(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	var sql string
	if p.Dialect == "mongodb" {
		sql = fmt.Sprintf(`{"dropIndexes": %q, "index": %q}`, op.Table, op.Index)
	} else {
		sql = fmt.Sprintf("DROP INDEX IF EXISTS %s;", op.Index)
	}
	return migrate.Operation{
		Kind: migrate.OpDropIndex, Table: op.Table, Index: op.Index,
		SQL:    sql,
		Reason: fmt.Sprintf("Inverse of add_index %s", op.Index),
	}, nil, nil
}

func (p *Planner) invertDropIndex(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	if op.IndexDef == nil {
		c := p.commentPrefix()
		stub := fmt.Sprintf("%s Cannot reconstruct dropped index %q on %s — fill in CREATE INDEX manually.", c, op.Index, op.Table)
		return migrate.Operation{
			Kind: migrate.OpAddIndex, Table: op.Table, Index: op.Index,
			SQL: stub, Reason: "Manual: original index definition missing",
		}, &ManualItem{Table: op.Table, Reason: "drop_index without IndexDef", Stub: stub}, nil
	}
	if p.Dialect == "mongodb" {
		keys := ""
		for i, c := range op.IndexDef.Columns {
			if i > 0 { keys += ", " }
			keys += fmt.Sprintf("%q: 1", c)
		}
		unique := ""
		if op.IndexDef.Unique { unique = `, "unique": true` }
		sql := fmt.Sprintf(`{"createIndexes": %q, "indexes": [{"key": {%s}, "name": %q%s}]}`,
			op.Table, keys, op.IndexDef.Name, unique)
		return migrate.Operation{
			Kind: migrate.OpAddIndex, Table: op.Table, Index: op.Index, IndexDef: op.IndexDef,
			SQL: sql, Reason: fmt.Sprintf("Inverse of drop_index %s", op.Index),
		}, nil, nil
	}
	uniq := ""
	if op.IndexDef.Unique { uniq = "UNIQUE " }
	ro := migrate.Operation{
		Kind: migrate.OpAddIndex, Table: op.Table, Index: op.Index, IndexDef: op.IndexDef,
		SQL: fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s);",
			uniq, op.Index, op.Table, strings.Join(op.IndexDef.Columns, ", ")),
		Reason: fmt.Sprintf("Inverse of drop_index %s", op.Index),
	}
	return ro, nil, nil
}

func (p *Planner) invertAddConstraint(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	name := ""
	if op.ConstraintDef != nil { name = op.ConstraintDef.Name }
	if p.Dialect == "mongodb" {
		return migrate.Operation{
			Kind: migrate.OpDropConstraint, Table: op.Table, ConstraintDef: op.ConstraintDef,
			SQL: "", Reason: fmt.Sprintf("Inverse of add_constraint %s (no-op for mongodb)", name),
		}, nil, nil
	}
	return migrate.Operation{
		Kind: migrate.OpDropConstraint, Table: op.Table, ConstraintDef: op.ConstraintDef,
		SQL: fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;", op.Table, name),
		Reason: fmt.Sprintf("Inverse of add_constraint %s", name),
	}, nil, nil
}

func (p *Planner) invertDropConstraint(op migrate.Operation) (migrate.Operation, *ManualItem, error) {
	if op.ConstraintDef == nil {
		c := p.commentPrefix()
		stub := fmt.Sprintf("%s Cannot reconstruct dropped constraint on %s — fill in ADD CONSTRAINT manually.", c, op.Table)
		return migrate.Operation{
			Kind: migrate.OpAddConstraint, Table: op.Table,
			SQL: stub, Reason: "Manual: original constraint definition missing",
		}, &ManualItem{Table: op.Table, Reason: "drop_constraint without ConstraintDef", Stub: stub}, nil
	}
	if p.Dialect == "mongodb" {
		return migrate.Operation{
			Kind: migrate.OpAddConstraint, Table: op.Table, ConstraintDef: op.ConstraintDef,
			SQL: "", Reason: fmt.Sprintf("Inverse of drop_constraint %s (no-op for mongodb)", op.ConstraintDef.Name),
		}, nil, nil
	}
	op2 := migrate.Operation{
		Kind: migrate.OpAddConstraint, Table: op.Table, ConstraintDef: op.ConstraintDef,
	}
	var sql string
	var err error
	switch p.Dialect {
	case "mysql":
		sql, err = codegen.RenderMySQL(&op2)
	case "sqlite":
		sql, err = codegen.RenderSQLite(&op2)
	default:
		sql, err = codegen.RenderPostgres(&op2)
	}
	if err != nil { return op2, nil, err }
	op2.SQL = sql
	op2.Reason = fmt.Sprintf("Inverse of drop_constraint %s", op.ConstraintDef.Name)
	return op2, nil, nil
}

func (p *Planner) renderDownBody(_ *migrate.DiffPlan, ops []migrate.Operation) string {
	if p.Dialect == "mongodb" {
		var items []string
		for _, op := range ops {
			if op.SQL == "" {
				continue
			}
			items = append(items, strings.TrimRight(op.SQL, "\n"))
		}
		if len(items) == 0 {
			return ""
		}
		return "[\n  " + strings.Join(items, ",\n  ") + "\n]\n"
	}

	var b strings.Builder
	for i, op := range ops {
		if op.SQL == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("-- [%d/%d] (rollback) %s: %s", i+1, len(ops), op.Kind, op.Table))
		if op.Column != "" {
			b.WriteString("." + op.Column)
		}
		b.WriteByte('\n')
		b.WriteString("-- Reason: " + op.Reason + "\n")
		b.WriteString(op.SQL)
		if !strings.HasSuffix(op.SQL, "\n") {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// replaceBody splices a new body into existing file content while preserving
// the header. For SQL files the header is comment lines; for JSON files the
// header is empty (command-only files get completely replaced).
func (p *Planner) replaceBody(orig, body string) string {
	c := p.commentPrefix()
	lines := strings.Split(orig, "\n")
	headerEnd := 0
	for i, l := range lines {
		if strings.HasPrefix(l, c) || strings.TrimSpace(l) == "" {
			headerEnd = i + 1
			continue
		}
		break
	}
	header := strings.Join(lines[:headerEnd], "\n")
	if header != "" {
		header += "\n"
	}
	out := header + body
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out
}
