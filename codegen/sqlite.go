package codegen

import (
	"fmt"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// RenderSQLite emits SQLite DDL for a single operation.
//
// SQLite has limited ALTER COLUMN support. Type changes are emitted as a
// comment + a no-op so the user can rewrite manually using the well-known
// table-rebuild pattern. We deliberately do NOT auto-emit a destructive
// CREATE/INSERT/DROP/RENAME sequence — that lives in a future phase.
func RenderSQLite(op *migrate.Operation) (string, error) {
	switch op.Kind {
	case migrate.OpAddTable:
		return renderSQLiteCreateTable(op.NewTable)
	case migrate.OpDropTable:
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;", op.Table), nil
	case migrate.OpAddColumn:
		return renderSQLiteAddColumn(op.Table, op.After), nil
	case migrate.OpDropColumn:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", op.Table, op.Column), nil
	case migrate.OpAlterColumn:
		return fmt.Sprintf(
			"-- SQLite does not support ALTER COLUMN directly.\n"+
				"-- Manual rewrite required for %s.%s.",
			op.Table, op.Column), nil
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
		return renderSQLiteAddIndex(op.Table, op.IndexDef), nil
	case migrate.OpDropIndex:
		return fmt.Sprintf("DROP INDEX IF EXISTS %s;", op.Index), nil
	case migrate.OpAddConstraint, migrate.OpDropConstraint:
		return fmt.Sprintf("-- SQLite does not support ADD/DROP CONSTRAINT separately for %s.", op.Table), nil
	}
	return "", fmt.Errorf("unsupported op kind: %s", op.Kind)
}

func renderSQLiteCreateTable(t *migrate.TableModel) (string, error) {
	if t == nil {
		return "", fmt.Errorf("nil table")
	}
	cols := orderedCols(t)
	maxName, maxType := 0, 0
	rendered := make([][3]string, 0, len(cols))
	for _, name := range cols {
		c := t.Columns[name]
		typeStr := sqliteColumnType(c)
		extras := sqliteColumnExtras(c)
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
		if !hasTablePK {
			c := t.Columns[r[0]]
			if c.IsPK {
				if extras != "" {
					extras += " "
				}
				if c.AutoIncrement {
					extras += "PRIMARY KEY AUTOINCREMENT"
				} else {
					extras += "PRIMARY KEY"
				}
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

func renderSQLiteAddColumn(table string, c *migrate.ColumnModel) string {
	parts := []string{c.Name, sqliteColumnType(c)}
	extrasParts := []string{}
	if !c.Nullable && !c.IsPK {
		extrasParts = append(extrasParts, "NOT NULL")
	}
	if c.Default != nil {
		def := *c.Default
		if isStringType(c.SQLType) && !looksSQLFunction(def) {
			def = "'" + def + "'"
		}
		extrasParts = append(extrasParts, "DEFAULT "+def)
	} else if !c.Nullable && !c.IsPK {
		extrasParts = append(extrasParts, "DEFAULT NULL")
	}
	if len(extrasParts) > 0 {
		parts = append(parts, strings.Join(extrasParts, " "))
	}
	// SQLite ADD COLUMN has some restrictions (no PK, no UNIQUE in ADD COLUMN)
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", table, strings.Join(parts, " "))
}

func renderSQLiteAddIndex(table string, i *migrate.IndexModel) string {
	if i == nil {
		return ""
	}
	uniq := ""
	if i.Unique {
		uniq = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s);", uniq, i.Name, table, strings.Join(i.Columns, ", "))
}

func sqliteColumnType(c *migrate.ColumnModel) string {
	// SQLite has dynamic typing but accepts most DDL types — keep as given.
	return c.SQLType
}

func sqliteColumnExtras(c *migrate.ColumnModel) string {
	var parts []string
	if !c.Nullable && !c.IsPK {
		parts = append(parts, "NOT NULL")
	}
	if c.Default != nil {
		def := *c.Default
		if isStringType(c.SQLType) && !looksSQLFunction(def) {
			def = "'" + def + "'"
		}
		parts = append(parts, "DEFAULT "+def)
	}
	return strings.Join(parts, " ")
}
