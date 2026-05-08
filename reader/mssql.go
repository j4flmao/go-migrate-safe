package reader

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type MSSQLReader struct {
	db *sql.DB
}

func NewMSSQL(db *sql.DB) *MSSQLReader {
	return &MSSQLReader{db: db}
}

func (r *MSSQLReader) ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error) {
	sm := migrate.NewSchemaModel(schema)

	rows, err := r.db.QueryContext(ctx,
		`SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES
		 WHERE TABLE_CATALOG = DB_NAME() AND TABLE_SCHEMA = 'dbo' AND TABLE_TYPE = 'BASE TABLE'
		 ORDER BY TABLE_NAME`)
	if err != nil {
		return nil, fmt.Errorf("mssql reader: list tables: %w", err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, tn := range tableNames {
		tbl := migrate.NewTableModel(tn)

		cols, order, err := readColumns(ctx, r.db,
			`SELECT COLUMN_NAME, DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT
			 FROM INFORMATION_SCHEMA.COLUMNS
			 WHERE TABLE_CATALOG = DB_NAME() AND TABLE_SCHEMA = 'dbo' AND TABLE_NAME = @p1
			 ORDER BY ORDINAL_POSITION`, tn)
		if err != nil {
			return nil, fmt.Errorf("mssql reader: read columns for %s: %w", tn, err)
		}
		tbl.Columns = cols
		tbl.ColumnOrder = order

		indexes, err := readMSSQLIndexes(ctx, r.db, tn)
		if err != nil {
			return nil, fmt.Errorf("mssql reader: read indexes for %s: %w", tn, err)
		}
		tbl.Indexes = indexes

		sm.Tables[tn] = tbl
	}
	return sm, nil
}

func readMSSQLIndexes(ctx context.Context, db *sql.DB, table string) (map[string]*migrate.IndexModel, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT i.name, c.name, i.is_unique
		 FROM sys.indexes i
		 JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		 JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		 WHERE i.object_id = OBJECT_ID(@p1) AND i.name IS NOT NULL AND i.is_primary_key = 0
		 ORDER BY i.name, ic.key_ordinal`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := map[string]*migrate.IndexModel{}
	for rows.Next() {
		var idx migrate.IndexModel
		var colName string
		if err := rows.Scan(&idx.Name, &colName, &idx.Unique); err != nil {
			return nil, err
		}
		if existing, ok := indexes[idx.Name]; ok {
			existing.Columns = append(existing.Columns, colName)
		} else {
			idx.Columns = []string{colName}
			indexes[idx.Name] = &idx
		}
	}
	return indexes, rows.Err()
}
