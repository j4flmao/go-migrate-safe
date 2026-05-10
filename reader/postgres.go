package reader

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type PostgresReader struct {
	db *sql.DB
}

func (r *PostgresReader) ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error) {
	if r.db == nil {
		return nil, fmt.Errorf("postgres reader: nil db")
	}
	sm := migrate.NewSchemaModel(schema)

	tableQuery := `SELECT table_name FROM information_schema.tables WHERE table_schema = $1 AND table_type = 'BASE TABLE' ORDER BY table_name`
	tRows, err := r.db.QueryContext(ctx, tableQuery, schema)
	if err != nil {
		return nil, fmt.Errorf("postgres reader: list tables: %w", err)
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
				c.column_name, 
				c.data_type, 
				c.is_nullable, 
				c.column_default,
				EXISTS (
					SELECT 1 FROM information_schema.key_column_usage kcu
					JOIN information_schema.table_constraints tc ON kcu.constraint_name = tc.constraint_name AND kcu.table_schema = tc.table_schema
					WHERE kcu.table_schema = c.table_schema AND kcu.table_name = c.table_name AND kcu.column_name = c.column_name AND tc.constraint_type = 'PRIMARY KEY'
				) as is_pk,
				(c.column_default LIKE 'nextval%') as is_auto
			 FROM information_schema.columns c
			 WHERE c.table_schema = $1 AND c.table_name = $2
			 ORDER BY c.ordinal_position`, schema, tn)
		if err != nil {
			return nil, fmt.Errorf("postgres reader: read columns for %s: %w", tn, err)
		}
		tbl.Columns = cols
		tbl.ColumnOrder = order

		indexes, err := readIndexes(ctx, r.db,
			`SELECT i.relname, a.attname, ix.indisunique
			 FROM pg_class t, pg_class i, pg_index ix, pg_attribute a
			 WHERE t.oid = ix.indrelid AND i.oid = ix.indexrelid
			 AND a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
			 AND t.relname = $1 AND t.relkind = 'r'`, tn)
		if err != nil {
			return nil, fmt.Errorf("postgres reader: read indexes for %s: %w", tn, err)
		}
		tbl.Indexes = indexes

		// Read Foreign Keys
		fks, err := readPostgresFKs(ctx, r.db, schema, tn)
		if err == nil {
			for name, fk := range fks {
				tbl.Constraints[name] = fk
			}
		}

		sm.Tables[tn] = tbl
	}
	return sm, nil
}

func readPostgresFKs(ctx context.Context, db *sql.DB, schema, table string) (map[string]*migrate.ConstraintModel, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT
			tc.constraint_name, 
			kcu.column_name, 
			ccu.table_name AS foreign_table_name,
			ccu.column_name AS foreign_column_name 
		FROM 
			information_schema.table_constraints AS tc 
			JOIN information_schema.key_column_usage AS kcu
			  ON tc.constraint_name = kcu.constraint_name
			  AND tc.table_schema = kcu.table_schema
			JOIN information_schema.constraint_column_usage AS ccu
			  ON ccu.constraint_name = tc.constraint_name
			  AND ccu.table_schema = tc.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY' 
		  AND tc.table_schema = $1
		  AND tc.table_name = $2`, schema, table)
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
