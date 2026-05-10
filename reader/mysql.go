package reader

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type MySQLReader struct {
	db *sql.DB
}

func (r *MySQLReader) ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error) {
	if r.db == nil {
		return nil, fmt.Errorf("mysql reader: nil db")
	}
	sm := migrate.NewSchemaModel(schema)

	tableQuery := `SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE' ORDER BY table_name`
	tRows, err := r.db.QueryContext(ctx, tableQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("mysql reader: list tables: %w", err)
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
		cols, order, err := readColumns(ctx, r.db,
			`SELECT 
				column_name, 
				data_type, 
				is_nullable, 
				column_default,
				(column_key = 'PRI') as is_pk,
				(extra = 'auto_increment') as is_auto
			 FROM information_schema.columns
			 WHERE table_schema = ? AND table_name = ?
			 ORDER BY ordinal_position`, schema, tn)
		if err != nil {
			return nil, fmt.Errorf("mysql reader: read columns for %s: %w", tn, err)
		}
		tbl.Columns = cols
		tbl.ColumnOrder = order

		indexes, err := readIndexes(ctx, r.db,
			`SELECT index_name, column_name, non_unique=0
			 FROM information_schema.statistics
			 WHERE table_schema = ? AND table_name = ?`, schema, tn)
		if err != nil {
			return nil, fmt.Errorf("mysql reader: read indexes for %s: %w", tn, err)
		}
		tbl.Indexes = indexes

		// Read Foreign Keys
		fks, err := readMySQLFKs(ctx, r.db, schema, tn)
		if err == nil {
			for name, fk := range fks {
				tbl.Constraints[name] = fk
			}
		}

		sm.Tables[tn] = tbl
	}
	return sm, nil
}

func readMySQLFKs(ctx context.Context, db *sql.DB, schema, table string) (map[string]*migrate.ConstraintModel, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT 
			constraint_name, 
			column_name, 
			referenced_table_name, 
			referenced_column_name
		 FROM information_schema.key_column_usage
		 WHERE table_schema = ? AND table_name = ? AND referenced_table_name IS NOT NULL`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fks := map[string]*migrate.ConstraintModel{}
	for rows.Next() {
		var name, col, refTable, refCol string
		if err := rows.Scan(&name, &col, &refTable, &refCol); err != nil {
			return nil, err
		}
		if _, ok := fks[name]; !ok {
			fks[name] = &migrate.ConstraintModel{
				Name:     name,
				Kind:     migrate.ConstraintForeignKey,
				RefTable: refTable,
			}
		}
		fks[name].Columns = append(fks[name].Columns, col)
		fks[name].RefColumns = append(fks[name].RefColumns, refCol)
	}
	return fks, nil
}
