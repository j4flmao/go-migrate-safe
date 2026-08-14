package codegen

import (
	"fmt"
	"sort"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// RenderPostgresBatch groups multiple operations into efficient batched statements.
func RenderPostgresBatch(ops []migrate.Operation) []string {
	if len(ops) == 0 {
		return nil
	}

	var results []string
	type alterGroup struct {
		table string
		parts []string
	}
	var currentAlter *alterGroup

	flush := func() {
		if currentAlter != nil && len(currentAlter.parts) > 0 {
			results = append(results, fmt.Sprintf("ALTER TABLE %s\n    %s;", currentAlter.table, strings.Join(currentAlter.parts, ",\n    ")))
			currentAlter = nil
		}
	}

	for i := range ops {
		op := &ops[i]
		canBatch := false
		var part string

		switch op.Kind {
		case migrate.OpAddColumn:
			canBatch = true
			part = "ADD COLUMN IF NOT EXISTS " + pgAddColumnPart(op.After)
		case migrate.OpDropColumn:
			canBatch = true
			part = fmt.Sprintf("DROP COLUMN IF EXISTS %s", op.Column)
		case migrate.OpAlterColumn:
			// OpAlterColumn might produce multiple sub-commands (TYPE, NULL, DEFAULT)
			canBatch = true
			parts := pgAlterColumnParts(op)
			if len(parts) > 0 {
				part = strings.Join(parts, ",\n    ")
			} else {
				canBatch = false // nothing to do
			}
		case migrate.OpRenameColumn:
			canBatch = true
			part = fmt.Sprintf("RENAME COLUMN %s TO %s", op.Before.Name, op.After.Name)
		case migrate.OpDropConstraint:
			canBatch = true
			name := ""
			if op.ConstraintDef != nil {
				name = op.ConstraintDef.Name
			}
			part = fmt.Sprintf("DROP CONSTRAINT IF EXISTS %s", name)
		}

		if canBatch {
			if currentAlter != nil && currentAlter.table != op.Table {
				flush()
			}
			if currentAlter == nil {
				currentAlter = &alterGroup{table: op.Table}
			}
			currentAlter.parts = append(currentAlter.parts, part)
		} else {
			flush()
			sql, _ := RenderPostgres(op)
			if sql != "" {
				results = append(results, sql)
			}
		}
	}
	flush()
	return results
}

func pgAddColumnPart(c *migrate.ColumnModel) string {
	parts := []string{c.Name, pgColumnType(c)}
	if extras := pgColumnExtras(c, true); extras != "" {
		parts = append(parts, extras)
	}
	return strings.Join(parts, " ")
}

func pgAlterColumnParts(op *migrate.Operation) []string {
	a, b := op.Before, op.After
	if a == nil || b == nil {
		return nil
	}
	var parts []string
	if a.SQLType != b.SQLType {
		parts = append(parts, fmt.Sprintf("ALTER COLUMN %s SET DATA TYPE %s", op.Column, b.SQLType))
	}
	if a.Nullable != b.Nullable {
		if b.Nullable {
			parts = append(parts, fmt.Sprintf("ALTER COLUMN %s DROP NOT NULL", op.Column))
		} else {
			parts = append(parts, fmt.Sprintf("ALTER COLUMN %s SET NOT NULL", op.Column))
		}
	}
	if !samePtr(a.Default, b.Default) {
		if b.Default == nil {
			parts = append(parts, fmt.Sprintf("ALTER COLUMN %s DROP DEFAULT", op.Column))
		} else {
			def := *b.Default
			if isStringType(b.SQLType) && !looksSQLFunction(def) {
				def = "'" + def + "'"
			}
			parts = append(parts, fmt.Sprintf("ALTER COLUMN %s SET DEFAULT %s", op.Column, def))
		}
	}
	return parts
}

// RenderPostgres emits PostgreSQL DDL for a single operation.
func RenderPostgres(op *migrate.Operation) (string, error) {
	switch op.Kind {
	case migrate.OpAddTable:
		return renderPGCreateTable(op.NewTable)
	case migrate.OpDropTable:
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;", op.Table), nil
	case migrate.OpAddColumn:
		return renderPGAddColumn(op.Table, op.After), nil
	case migrate.OpDropColumn:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s;", op.Table, op.Column), nil
	case migrate.OpAlterColumn:
		return renderPGAlterColumn(op), nil
	case migrate.OpRenameColumn:
		if op.Before == nil || op.After == nil {
			return "", fmt.Errorf("rename op requires Before and After")
		}
		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", op.Table, op.Before.Name, op.After.Name), nil
	case migrate.OpRenameTable:
		if op.NewTable == nil {
			return "", fmt.Errorf("rename table op requires NewTable")
		}
		return fmt.Sprintf("ALTER TABLE %s RENAME TO %s;", op.Table, op.NewTable.Name), nil
	case migrate.OpAddIndex:
		return renderPGAddIndex(op.Table, op.IndexDef), nil
	case migrate.OpDropIndex:
		return fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s;", op.Index), nil
	case migrate.OpAddConstraint:
		return renderPGAddConstraint(op.Table, op.ConstraintDef)
	case migrate.OpDropConstraint:
		name := ""
		if op.ConstraintDef != nil {
			name = op.ConstraintDef.Name
		}
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;", op.Table, name), nil
	}
	return "", fmt.Errorf("unsupported op kind: %s", op.Kind)
}

func renderPGCreateTable(t *migrate.TableModel) (string, error) {
	if t == nil {
		return "", fmt.Errorf("nil table")
	}
	cols := orderedCols(t)

	maxName, maxType := 0, 0
	rendered := make([][3]string, 0, len(cols))
	for _, name := range cols {
		c := t.Columns[name]
		typeStr := pgColumnType(c)
		extras := pgColumnExtras(c, false)
		rendered = append(rendered, [3]string{name, typeStr, extras})
		if len(name) > maxName {
			maxName = len(name)
		}
		if len(typeStr) > maxType {
			maxType = len(typeStr)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s (\n", t.Name)
	hasTablePK := len(pkCols(t)) > 1
	for i, r := range rendered {
		end := ","
		if i == len(rendered)-1 && !hasTablePK {
			end = ""
		}
		extras := r[2]
		// Inline PRIMARY KEY for single-column PK case.
		if !hasTablePK {
			c := t.Columns[r[0]]
			if c.IsPK {
				if extras != "" {
					extras += " "
				}
				extras += "PRIMARY KEY"
			}
		}
		if extras != "" {
			extras = " " + extras
		}
		fmt.Fprintf(&b, "    %-*s  %-*s%s%s\n", maxName, r[0], maxType, r[1], extras, end)
	}
	if hasTablePK {
		fmt.Fprintf(&b, "    PRIMARY KEY (%s)\n", strings.Join(pkCols(t), ", "))
	}
	b.WriteString(");")
	return b.String(), nil
}

func renderPGAddColumn(table string, c *migrate.ColumnModel) string {
	parts := []string{c.Name, pgColumnType(c)}
	if extras := pgColumnExtras(c, true); extras != "" {
		parts = append(parts, extras)
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s;", table, strings.Join(parts, " "))
}

func renderPGAlterColumn(op *migrate.Operation) string {
	a, b := op.Before, op.After
	if a == nil || b == nil {
		return fmt.Sprintf("-- alter column %s.%s requires Before/After", op.Table, op.Column)
	}
	var stmts []string
	if a.SQLType != b.SQLType {
		stmts = append(stmts,
			fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DATA TYPE %s;", op.Table, op.Column, b.SQLType))
	}
	if a.Nullable != b.Nullable {
		if b.Nullable {
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;", op.Table, op.Column))
		} else {
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;", op.Table, op.Column))
		}
	}
	if !samePtr(a.Default, b.Default) {
		if b.Default == nil {
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s DROP DEFAULT;", op.Table, op.Column))
		} else {
			def := *b.Default
			if isStringType(b.SQLType) && !looksSQLFunction(def) {
				def = "'" + def + "'"
			}
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s;", op.Table, op.Column, def))
		}
	}
	return strings.Join(stmts, "\n")
}

func renderPGAddIndex(table string, i *migrate.IndexModel) string {
	if i == nil {
		return ""
	}
	uniq := ""
	if i.Unique {
		uniq = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX CONCURRENTLY IF NOT EXISTS %s ON %s (%s);", uniq, i.Name, table, strings.Join(i.Columns, ", "))
}

func renderPGAddConstraint(table string, c *migrate.ConstraintModel) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nil constraint")
	}
	switch c.Kind {
	case migrate.ConstraintForeignKey:
		return fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s) NOT VALID;\nALTER TABLE %s VALIDATE CONSTRAINT %s;",
			table, c.Name, strings.Join(c.Columns, ", "), c.RefTable, strings.Join(c.RefColumns, ", "),
			table, c.Name,
		), nil
	case migrate.ConstraintUnique:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);",
			table, c.Name, strings.Join(c.Columns, ", "),
		), nil
	case migrate.ConstraintCheck:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s);", table, c.Name, c.CheckExpr), nil
	case migrate.ConstraintPrimaryKey:
		return fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s PRIMARY KEY (%s);", table, c.Name, strings.Join(c.Columns, ", ")), nil
	}
	return "", fmt.Errorf("unsupported constraint kind: %s", c.Kind)
}

func pgColumnType(c *migrate.ColumnModel) string {
	if c.AutoIncrement {
		switch c.SQLType {
		case "BIGINT":
			return "BIGSERIAL"
		case "INTEGER":
			return "SERIAL"
		case "SMALLINT":
			return "SMALLSERIAL"
		}
	}
	return c.SQLType
}

func pgColumnExtras(c *migrate.ColumnModel, isAdd bool) string {
	var parts []string
	if !c.Nullable && !c.AutoIncrement {
		parts = append(parts, "NOT NULL")
	}
	if c.Default != nil {
		def := *c.Default
		if isStringType(c.SQLType) && !looksSQLFunction(def) {
			def = "'" + def + "'"
		}
		parts = append(parts, "DEFAULT "+def)
	} else if isAdd && !c.Nullable {
		parts = append(parts, "DEFAULT NULL")
	}
	return strings.Join(parts, " ")
}

func pkCols(t *migrate.TableModel) []string {
	var out []string
	for _, n := range orderedCols(t) {
		if t.Columns[n].IsPK {
			out = append(out, n)
		}
	}
	return out
}

func orderedCols(t *migrate.TableModel) []string {
	if len(t.ColumnOrder) == len(t.Columns) {
		return t.ColumnOrder
	}
	out := make([]string, 0, len(t.Columns))
	for k := range t.Columns {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func samePtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
