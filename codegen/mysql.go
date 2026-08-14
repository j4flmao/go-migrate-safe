package codegen

import (
	"fmt"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// RenderMySQL emits MySQL DDL for a single operation.
func RenderMySQLBatch(ops []migrate.Operation) []string {
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
			part = "ADD COLUMN " + mysqlAddColumnPart(op.After)
		case migrate.OpDropColumn:
			canBatch = true
			part = fmt.Sprintf("DROP COLUMN %s", op.Column)
		case migrate.OpAlterColumn:
			canBatch = true
			part = "MODIFY COLUMN " + mysqlAlterColumnPart(op)
		case migrate.OpRenameColumn:
			canBatch = true
			part = fmt.Sprintf("RENAME COLUMN %s TO %s", op.Before.Name, op.After.Name)
		case migrate.OpDropConstraint:
			canBatch = true
			name := ""
			if op.ConstraintDef != nil {
				name = op.ConstraintDef.Name
			}
			part = fmt.Sprintf("DROP CONSTRAINT %s", name)
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
			sql, _ := RenderMySQL(op)
			if sql != "" {
				results = append(results, sql)
			}
		}
	}
	flush()
	return results
}

func mysqlAddColumnPart(c *migrate.ColumnModel) string {
	parts := []string{c.Name, mysqlColumnType(c)}
	extrasParts := []string{}
	if !c.Nullable {
		extrasParts = append(extrasParts, "NOT NULL")
	}
	if c.Default != nil {
		extrasParts = append(extrasParts, "DEFAULT "+mysqlDefaultValue(c))
	} else if !c.Nullable {
		extrasParts = append(extrasParts, "DEFAULT NULL")
	}
	if len(extrasParts) > 0 {
		parts = append(parts, strings.Join(extrasParts, " "))
	}
	return strings.Join(parts, " ")
}

func mysqlAlterColumnPart(op *migrate.Operation) string {
	b := op.After
	parts := []string{op.Column, b.SQLType}
	if !b.Nullable {
		parts = append(parts, "NOT NULL")
	} else {
		parts = append(parts, "NULL")
	}
	if b.Default != nil {
		def := *b.Default
		if isStringType(b.SQLType) && !looksSQLFunction(def) {
			def = "'" + def + "'"
		}
		parts = append(parts, "DEFAULT "+def)
	}
	return strings.Join(parts, " ")
}

// RenderMySQL emits MySQL DDL for a single operation.
func RenderMySQL(op *migrate.Operation) (string, error) {
	switch op.Kind {
	case migrate.OpAddTable:
		return renderMySQLCreateTable(op.NewTable)
	case migrate.OpDropTable:
		return fmt.Sprintf("DROP TABLE IF EXISTS %s;", op.Table), nil
	case migrate.OpAddColumn:
		return renderMySQLAddColumn(op.Table, op.After), nil
	case migrate.OpDropColumn:
		return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s;", op.Table, op.Column), nil
	case migrate.OpAlterColumn:
		return renderMySQLAlterColumn(op), nil
	case migrate.OpRenameColumn:
		if op.Before == nil || op.After == nil {
			return "", fmt.Errorf("rename op requires Before and After")
		}
		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s;", op.Table, op.Before.Name, op.After.Name), nil
	case migrate.OpRenameTable:
		if op.NewTable == nil {
			return "", fmt.Errorf("rename table op requires NewTable")
		}
		return fmt.Sprintf("RENAME TABLE %s TO %s;", op.Table, op.NewTable.Name), nil
	case migrate.OpAddIndex:
		return renderMySQLAddIndex(op.Table, op.IndexDef), nil
	case migrate.OpDropIndex:
		return fmt.Sprintf("ALTER TABLE %s DROP INDEX %s, ALGORITHM=INPLACE, LOCK=NONE;", op.Table, op.Index), nil
	case migrate.OpAddConstraint:
		return renderMySQLAddConstraint(op.Table, op.ConstraintDef)
	case migrate.OpDropConstraint:
		name := ""
		if op.ConstraintDef != nil {
			name = op.ConstraintDef.Name
		}
		return fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s;", op.Table, name), nil
	}
	return "", fmt.Errorf("unsupported op kind: %s", op.Kind)
}

func renderMySQLCreateTable(t *migrate.TableModel) (string, error) {
	if t == nil {
		return "", fmt.Errorf("nil table")
	}
	cols := orderedCols(t)
	maxName, maxType := 0, 0
	rendered := make([][3]string, 0, len(cols))
	for _, name := range cols {
		c := t.Columns[name]
		typeStr := mysqlColumnType(c)
		extras := mysqlColumnExtras(c)
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
	b.WriteString(") ENGINE=InnoDB;")
	return b.String(), nil
}

func renderMySQLAddColumn(table string, c *migrate.ColumnModel) string {
	parts := []string{c.Name, mysqlColumnType(c)}
	extrasParts := []string{}
	if !c.Nullable {
		extrasParts = append(extrasParts, "NOT NULL")
	}
	if c.Default != nil {
		extrasParts = append(extrasParts, "DEFAULT "+mysqlDefaultValue(c))
	} else if !c.Nullable {
		extrasParts = append(extrasParts, "DEFAULT NULL")
	}
	if len(extrasParts) > 0 {
		parts = append(parts, strings.Join(extrasParts, " "))
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s;", table, strings.Join(parts, " "))
}

func renderMySQLAlterColumn(op *migrate.Operation) string {
	a, b := op.Before, op.After
	if a == nil || b == nil {
		return fmt.Sprintf("-- alter column %s.%s requires Before/After", op.Table, op.Column)
	}
	parts := []string{op.Column, b.SQLType}
	if !b.Nullable {
		parts = append(parts, "NOT NULL")
	} else {
		parts = append(parts, "NULL")
	}
	if b.Default != nil {
		def := *b.Default
		if isStringType(b.SQLType) && !looksSQLFunction(def) {
			def = "'" + def + "'"
		}
		parts = append(parts, "DEFAULT "+def)
	}
	return fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s;", op.Table, strings.Join(parts, " "))
}

func renderMySQLAddIndex(table string, i *migrate.IndexModel) string {
	if i == nil {
		return ""
	}
	uniq := ""
	if i.Unique {
		uniq = "UNIQUE "
	}
	return fmt.Sprintf("ALTER TABLE %s ADD %sINDEX %s (%s), ALGORITHM=INPLACE, LOCK=NONE;", table, uniq, i.Name, strings.Join(i.Columns, ", "))
}

func renderMySQLAddConstraint(table string, c *migrate.ConstraintModel) (string, error) {
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

func mysqlColumnType(c *migrate.ColumnModel) string {
	sqlType := c.SQLType
	if c.Default != nil && !looksSQLFunction(*c.Default) && isStringType(sqlType) {
		sqlType = "VARCHAR(255)"
	}
	if c.AutoIncrement {
		return sqlType + " AUTO_INCREMENT"
	}
	return sqlType
}

func mysqlColumnExtras(c *migrate.ColumnModel) string {
	var parts []string
	if !c.Nullable {
		parts = append(parts, "NOT NULL")
	}
	if c.Default != nil {
		parts = append(parts, "DEFAULT "+mysqlDefaultValue(c))
	}
	return strings.Join(parts, " ")
}

func mysqlDefaultValue(c *migrate.ColumnModel) string {
	v := *c.Default
	if isStringType(c.SQLType) && !looksSQLFunction(v) {
		return "'" + v + "'"
	}
	return v
}

func isStringType(sqlType string) bool {
	t := strings.ToUpper(sqlType)
	return strings.HasPrefix(t, "TEXT") || strings.HasPrefix(t, "VARCHAR") || strings.HasPrefix(t, "CHAR") || strings.HasPrefix(t, "NVARCHAR") || strings.HasPrefix(t, "NCHAR")
}

func looksSQLFunction(val string) bool {
	upper := strings.ToUpper(val)
	switch upper {
	case "CURRENT_TIMESTAMP", "CURRENT_DATE", "CURRENT_TIME", "LOCALTIME", "LOCALTIMESTAMP", "NOW", "UUID":
		return true
	}
	if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
		return true
	}
	return strings.Contains(upper, "(")
}
