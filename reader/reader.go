package reader

import (
	"context"
	"database/sql"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

type Reader interface {
	ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error)
}

func NewPostgres(db *sql.DB) *PostgresReader {
	return &PostgresReader{db: db}
}

func NewMySQL(db *sql.DB) *MySQLReader {
	return &MySQLReader{db: db}
}

func NewSQLite(db *sql.DB) *SQLiteReader {
	return &SQLiteReader{db: db}
}

func readColumns(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]*migrate.ColumnModel, []string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns := map[string]*migrate.ColumnModel{}
	var order []string
	for rows.Next() {
		var nullable string
		var defaultPtr *string
		var colName, sqlType string
		if err := rows.Scan(&colName, &sqlType, &nullable, &defaultPtr); err != nil {
			return nil, nil, err
		}
		columns[colName] = &migrate.ColumnModel{
			Name:     colName,
			SQLType:  sqlType,
			Nullable: nullable == "YES",
			Default:  defaultPtr,
		}
		order = append(order, colName)
	}
	return columns, order, rows.Err()
}

func readIndexes(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]*migrate.IndexModel, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := map[string]*migrate.IndexModel{}
	for rows.Next() {
		var idx migrate.IndexModel
		var unique bool
		var colName string
		if err := rows.Scan(&idx.Name, &colName, &unique); err != nil {
			return nil, err
		}
		if existing, ok := indexes[idx.Name]; ok {
			existing.Columns = append(existing.Columns, colName)
		} else {
			idx.Unique = unique
			idx.Columns = []string{colName}
			indexes[idx.Name] = &idx
		}
	}
	return indexes, rows.Err()
}
