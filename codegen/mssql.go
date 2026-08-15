package codegen

import (
	"fmt"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

func RenderMSSQL(op *migrate.Operation) (string, error) {
	switch op.Kind {
	case migrate.OpAddTable:
		return renderMSSQLCreateTable(op.NewTable)
	case migrate.OpDropTable:
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;", op.Table), nil
	case migrate.OpAddColumn:
		return renderMSSQLAddColumn(op.Table, op.After), nil
	case migrate.OpDropColumn:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", op.Table, op.Column), nil
	case migrate.OpAlterColumn:
		return renderMSSQLAlterColumn(op), nil
	case migrate.OpRenameColumn:
		if op.Before == nil || op.After == nil {
			return "", fmt.Errorf("rename op requires Before and After")
		}
		return fmt.Sprintf("EXEC sp_rename '%s.%s', '%s', 'COLUMN';", op.Table, op.Before.Name, op.After.Name), nil
	case migrate.OpRenameTable:
		if op.NewTable == nil {
			return "", fmt.Errorf("rename table op requires NewTable")
		}
		return fmt.Sprintf("EXEC sp_rename '%s', '%s';", op.Table, op.NewTable.Name), nil
	case migrate.OpAddIndex:
		return renderMSSQLAddIndex(op.Table, op.IndexDef), nil
	case migrate.OpDropIndex:
		return fmt.Sprintf("DROP INDEX %s ON %s;", op.Index, op.Table), nil
	case migrate.OpAddConstraint:
		return renderMSSQLAddConstraint(op.Table, op.ConstraintDef)
	case migrate.OpDropConstraint:
		name := ""
		if op.ConstraintDef != nil {
			name = op.ConstraintDef.Name
		}
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", op.Table, name), nil
	}
	return "", fmt.Errorf("unsupported op kind: %s", op.Kind)
}

func renderMSSQLCreateTable(t *migrate.TableModel) (string, error) {
	if t == nil {
		return "", fmt.Errorf("nil table")
	}
	cols := orderedCols(t)
	maxName, maxType := 0, 0
	rendered := make([][3]string, 0, len(cols))
	for _, name := range cols {
		c := t.Columns[name]
		typeStr := mssqlColumnType(c)
		extras := mssqlColumnExtras(c)
		rendered = append(rendered, [3]string{name, typeStr, extras})
		if len(name) > maxName {
			maxName = len(name)
		}
		if len(typeStr) > maxType {
			maxType = len(typeStr)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %s (\n", t.Name)
	hasPK := len(pkCols(t)) > 0
	for i, r := range rendered {
		end := ","
		if i == len(rendered)-1 && !hasPK {
			end = ""
		}
		extras := r[2]
		if extras != "" {
			extras = " " + extras
		}
		fmt.Fprintf(&b, "    %-*s  %-*s%s%s\n", maxName, r[0], maxType, r[1], extras, end)
	}
	if hasPK {
		fmt.Fprintf(&b, "    PRIMARY KEY (%s)\n", strings.Join(pkCols(t), ", "))
	}
	b.WriteString(");")
	return b.String(), nil
}

func renderMSSQLAddColumn(table string, c *migrate.ColumnModel) string {
	parts := []string{c.Name, mssqlColumnType(c)}
	extrasParts := []string{}
	if !c.Nullable && !c.AutoIncrement {
		extrasParts = append(extrasParts, "NOT NULL")
	}
	if c.Default != nil {
		def := *c.Default
		if isStringType(c.SQLType) && !looksSQLFunction(def) {
			def = "'" + def + "'"
		}
		extrasParts = append(extrasParts, "DEFAULT "+def)
	} else if !c.Nullable && !c.AutoIncrement {
		extrasParts = append(extrasParts, "DEFAULT NULL")
	}
	if len(extrasParts) > 0 {
		parts = append(parts, strings.Join(extrasParts, " "))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD %s;", table, strings.Join(parts, " "))
}

func renderMSSQLAlterColumn(op *migrate.Operation) string {
	a, b := op.Before, op.After
	if a == nil || b == nil {
		return fmt.Sprintf("-- alter column %s.%s requires Before/After", op.Table, op.Column)
	}
	var stmts []string
	if a.SQLType != b.SQLType || a.Nullable != b.Nullable {
		parts := []string{op.Column, mssqlColumnType(b)}
		if !b.Nullable {
			parts = append(parts, "NOT NULL")
		} else {
			parts = append(parts, "NULL")
		}
		stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s;", op.Table, strings.Join(parts, " ")))
	}
	if !samePtr(a.Default, b.Default) {
		if b.Default == nil {
			stmts = append(stmts, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT DF_%s_%s;", op.Table, op.Table, op.Column))
		} else {
			def := *b.Default
			if isStringType(b.SQLType) && !looksSQLFunction(def) {
				def = "'" + def + "'"
			}
			stmts = append(stmts,
				fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT DF_%s_%s DEFAULT %s FOR %s;",
					op.Table, op.Table, op.Column, def, op.Column))
		}
	}
	return strings.Join(stmts, "\n")
}

func renderMSSQLAddIndex(table string, i *migrate.IndexModel) string {
	if i == nil {
		return ""
	}
	uniq := ""
	if i.Unique {
		uniq = "UNIQUE "
	}
	return fmt.Sprintf("CREATE %sINDEX %s ON %s (%s);", uniq, i.Name, table, strings.Join(i.Columns, ", "))
}

func renderMSSQLAddConstraint(table string, c *migrate.ConstraintModel) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nil constraint")
	}
	switch c.Kind {
	case migrate.ConstraintForeignKey:
		return fmt.Sprintf(
			"ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s);",
			table, c.Name, strings.Join(c.Columns, ", "), c.RefTable, strings.Join(c.RefColumns, ", "),
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

func mssqlColumnType(c *migrate.ColumnModel) string {
	sqlType := c.SQLType
	if c.Default != nil && !looksSQLFunction(*c.Default) && isStringType(sqlType) {
		sqlType = "NVARCHAR(255)"
	}
	if c.AutoIncrement {
		switch sqlType {
		case "BIGINT":
			return "BIGINT IDENTITY(1,1)"
		case "INTEGER", "INT":
			return "INT IDENTITY(1,1)"
		case "SMALLINT":
			return "SMALLINT IDENTITY(1,1)"
		}
	}
	if strings.ToUpper(sqlType) == "TEXT" || strings.ToUpper(sqlType) == "LONGTEXT" || strings.ToUpper(sqlType) == "MEDIUMTEXT" {
		return "NVARCHAR(MAX)"
	}
	if strings.ToUpper(sqlType) == "DOUBLE" {
		return "FLOAT"
	}
	if strings.ToUpper(sqlType) == "DATETIME" {
		return "DATETIME2"
	}
	if strings.ToUpper(sqlType) == "TINYINT(1)" || strings.ToUpper(sqlType) == "BOOLEAN" {
		return "BIT"
	}
	return sqlType
}

func mssqlColumnExtras(c *migrate.ColumnModel) string {
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
	}
	return strings.Join(parts, " ")
}
