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
			`SELECT column_name, data_type, is_nullable, column_default
			 FROM information_schema.columns
			 WHERE table_schema = $1 AND table_name = $2
			 ORDER BY ordinal_position`, schema, tn)
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

		sm.Tables[tn] = tbl
	}
	return sm, nil
}
