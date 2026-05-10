package reader

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type SQLiteReader struct {
	db *sql.DB
}

func (r *SQLiteReader) ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error) {
	if r.db == nil {
		return nil, fmt.Errorf("sqlite reader: nil db")
	}
	sm := migrate.NewSchemaModel(schema)

	tRows, err := r.db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("sqlite reader: list tables: %w", err)
	}
	defer tRows.Close()

	var tableNames []string
	for tRows.Next() {
		var name string
		if err := tRows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := tRows.Err(); err != nil {
		return nil, err
	}

	for _, tn := range tableNames {
		tbl := migrate.NewTableModel(tn)

		pRows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT cid, name, type, "notnull", dflt_value, pk FROM pragma_table_info('%s')`, tn))
		if err != nil {
			return nil, fmt.Errorf("sqlite reader: read columns for %s: %w", tn, err)
		}
		var order []string
		for pRows.Next() {
			var cid int
			var colName, sqlType string
			var notnull int
			var defaultPtr *string
			var pk int
			if err := pRows.Scan(&cid, &colName, &sqlType, &notnull, &defaultPtr, &pk); err != nil {
				pRows.Close()
				return nil, err
			}
			col := &migrate.ColumnModel{
				Name:     colName,
				SQLType:  strings.ToUpper(sqlType),
				Nullable: notnull == 0,
				Default:  defaultPtr,
				IsPK:     pk != 0,
			}
			tbl.Columns[colName] = col
			order = append(order, colName)
		}
		pRows.Close()
		tbl.ColumnOrder = order

		iRows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT name, "unique" FROM pragma_index_list('%s') WHERE origin != 'pk'`, tn))
		if err == nil {
			var idxNames []string
			var idxUnique []bool
			for iRows.Next() {
				var name string
				var unique int
				if err := iRows.Scan(&name, &unique); err != nil {
					iRows.Close()
					break
				}
				idxNames = append(idxNames, name)
				idxUnique = append(idxUnique, unique != 0)
			}
			iRows.Close()
			for i, idxName := range idxNames {
				cRows, err := r.db.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_index_info('%s')`, idxName))
				if err != nil {
					continue
				}
				var cols []string
				for cRows.Next() {
					var colName string
					if err := cRows.Scan(&colName); err != nil {
						break
					}
					cols = append(cols, colName)
				}
				cRows.Close()
				tbl.Indexes[idxName] = &migrate.IndexModel{
					Name:    idxName,
					Columns: cols,
					Unique:  idxUnique[i],
				}
			}
		}

		// Read Foreign Keys
		fks, err := readSQLiteFKs(ctx, r.db, tn)
		if err == nil {
			for name, fk := range fks {
				tbl.Constraints[name] = fk
			}
		}

		sm.Tables[tn] = tbl
	}
	return sm, nil
}

func readSQLiteFKs(ctx context.Context, db *sql.DB, table string) (map[string]*migrate.ConstraintModel, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list('%s')", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fks := map[string]*migrate.ConstraintModel{}
	for rows.Next() {
		var id, seq int
		var refTable, fromCol, toCol, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}

		name := fmt.Sprintf("fk_%s_%s_%d", table, refTable, id)
		if _, ok := fks[name]; !ok {
			fks[name] = &migrate.ConstraintModel{
				Name:     name,
				Kind:     migrate.ConstraintForeignKey,
				RefTable: refTable,
			}
		}
		fks[name].Columns = append(fks[name].Columns, fromCol)
		fks[name].RefColumns = append(fks[name].RefColumns, toCol)
	}
	return fks, nil
}
