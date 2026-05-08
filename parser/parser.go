package parser

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type Parser struct {
	Dialect Dialect
	Schema  string
}

func New(dialect Dialect, schema string) *Parser {
	return &Parser{Dialect: dialect, Schema: schema}
}

func (p *Parser) Parse(models ...any) (*migrate.SchemaModel, error) {
	if len(models) == 0 {
		return nil, fmt.Errorf("parser.Parse: %w", migrate.ErrNoModels)
	}
	sm := migrate.NewSchemaModel(p.Schema)
	for _, m := range models {
		t := reflect.TypeOf(m)
		if t == nil {
			return nil, fmt.Errorf("parser.Parse: nil model passed")
		}
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return nil, fmt.Errorf("parser.Parse: expected struct, got %s", t.Kind())
		}
		tbl, err := p.parseStruct(t)
		if err != nil {
			return nil, fmt.Errorf("parser.Parse %s: %w", t.Name(), err)
		}
		if _, exists := sm.Tables[tbl.Name]; exists {
			return nil, fmt.Errorf("parser.Parse: duplicate table %q", tbl.Name)
		}
		sm.Tables[tbl.Name] = tbl
	}
	return sm, nil
}

func (p *Parser) parseStruct(t reflect.Type) (*migrate.TableModel, error) {
	tableName := PluralizeTable(t.Name())

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "_" {
			tag, err := ParseTag(f.Tag.Get("db"))
			if err != nil {
				return nil, err
			}
			if tag.TableOverride != "" {
				tableName = tag.TableOverride
			}
		}
	}
	if err := ValidateIdentifier(tableName); err != nil {
		return nil, fmt.Errorf("table %s: %w", t.Name(), err)
	}

	tbl := migrate.NewTableModel(tableName)

	type group struct {
		Name   string
		Cols   []string
		Order  []int
		Unique bool
		IsPK   bool
	}
	indexGroups := map[string]*group{}
	uniqueGroups := map[string]*group{}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == "_" || !f.IsExported() {
			continue
		}
		tag, err := ParseTag(f.Tag.Get("db"))
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		if tag.Ignore {
			continue
		}

		colName := tag.ColumnName
		if colName == "" {
			colName = SnakeCase(f.Name)
		}
		if err := ValidateIdentifier(colName); err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}

		col := &migrate.ColumnModel{
			Name:          colName,
			IsPK:          tag.IsPK,
			AutoIncrement: tag.AutoIncrement,
			Default:       tag.Default,
			Size:          tag.Size,
			Precision:     tag.Precision,
			Scale:         tag.Scale,
		}

		if tag.TypeOverride != "" {
			col.SQLType = NormalizeType(tag.TypeOverride)
		} else {
			s, err := goSQLType(f.Type, p.Dialect)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", f.Name, err)
			}
			if tag.Size != nil && (s == "TEXT" || s == "VARCHAR" || strings.HasPrefix(s, "TEXT")) {
				s = fmt.Sprintf("VARCHAR(%d)", *tag.Size)
			}
			col.SQLType = s
		}

		switch {
		case tag.Nullable:
			col.Nullable = true
		case tag.NotNull:
			col.Nullable = false
		default:
			col.Nullable = isNullableGoType(f.Type)
		}
		if col.IsPK {
			col.Nullable = false
		}

		tbl.Columns[col.Name] = col
		tbl.ColumnOrder = append(tbl.ColumnOrder, col.Name)

		if tag.IsPK {
			pkName := "pk_" + tableName
			if c, ok := tbl.Constraints[pkName]; ok {
				c.Columns = append(c.Columns, col.Name)
			} else {
				tbl.Constraints[pkName] = &migrate.ConstraintModel{
					Name:    pkName,
					Kind:    migrate.ConstraintPrimaryKey,
					Columns: []string{col.Name},
				}
			}
		}

		if tag.Index && tag.IndexName == "" {
			name := fmt.Sprintf("idx_%s_%s", tableName, col.Name)
			tbl.Indexes[name] = &migrate.IndexModel{Name: name, Columns: []string{col.Name}}
		}
		if tag.Unique && tag.UniqueName == "" {
			name := fmt.Sprintf("uniq_%s_%s", tableName, col.Name)
			tbl.Indexes[name] = &migrate.IndexModel{Name: name, Columns: []string{col.Name}, Unique: true}
		}
		if tag.IndexName != "" {
			g := indexGroups[tag.IndexName]
			if g == nil {
				g = &group{Name: tag.IndexName}
				indexGroups[tag.IndexName] = g
			}
			g.Cols = append(g.Cols, col.Name)
			g.Order = append(g.Order, i)
		}
		if tag.UniqueName != "" {
			g := uniqueGroups[tag.UniqueName]
			if g == nil {
				g = &group{Name: tag.UniqueName, Unique: true}
				uniqueGroups[tag.UniqueName] = g
			}
			g.Cols = append(g.Cols, col.Name)
			g.Order = append(g.Order, i)
		}

		if tag.FKRefTable != "" {
			fkName := fmt.Sprintf("fk_%s_%s", tableName, tag.FKRefTable)
			tbl.Constraints[fkName] = &migrate.ConstraintModel{
				Name:       fkName,
				Kind:       migrate.ConstraintForeignKey,
				Columns:    []string{col.Name},
				RefTable:   tag.FKRefTable,
				RefColumns: []string{tag.FKRefColumn},
			}
		}
	}

	for _, g := range indexGroups {
		tbl.Indexes[g.Name] = &migrate.IndexModel{Name: g.Name, Columns: append([]string{}, g.Cols...)}
	}
	for _, g := range uniqueGroups {
		tbl.Indexes[g.Name] = &migrate.IndexModel{Name: g.Name, Columns: append([]string{}, g.Cols...), Unique: true}
		tbl.Constraints[g.Name] = &migrate.ConstraintModel{
			Name:    g.Name,
			Kind:    migrate.ConstraintUnique,
			Columns: append([]string{}, g.Cols...),
		}
	}

	return tbl, nil
}

func isNullableGoType(t reflect.Type) bool {
	if t.Kind() == reflect.Ptr {
		return true
	}
	if t.PkgPath() == "database/sql" {
		switch t.Name() {
		case "NullString", "NullInt64", "NullInt32", "NullBool", "NullFloat64", "NullTime":
			return true
		}
	}
	return false
}
