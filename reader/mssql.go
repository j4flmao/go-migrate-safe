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
			`SELECT 
				c.COLUMN_NAME, 
				c.DATA_TYPE, 
				c.IS_NULLABLE, 
				c.COLUMN_DEFAULT,
				CAST(CASE WHEN pk.COLUMN_NAME IS NOT NULL THEN 1 ELSE 0 END AS BIT) as IS_PK,
				CAST(COLUMNPROPERTY(OBJECT_ID(c.TABLE_SCHEMA + '.' + c.TABLE_NAME), c.COLUMN_NAME, 'IsIdentity') AS BIT) as IS_AUTO
			 FROM INFORMATION_SCHEMA.COLUMNS c
			 LEFT JOIN (
				SELECT ku.TABLE_CATALOG, ku.TABLE_SCHEMA, ku.TABLE_NAME, ku.COLUMN_NAME
				FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
				JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME AND tc.TABLE_SCHEMA = ku.TABLE_SCHEMA
				WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
			 ) pk ON  c.TABLE_CATALOG = pk.TABLE_CATALOG AND c.TABLE_SCHEMA = pk.TABLE_SCHEMA AND c.TABLE_NAME = pk.TABLE_NAME AND c.COLUMN_NAME = pk.COLUMN_NAME
			 WHERE c.TABLE_CATALOG = DB_NAME() AND c.TABLE_SCHEMA = 'dbo' AND c.TABLE_NAME = @p1
			 ORDER BY c.ORDINAL_POSITION`, tn)
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

		// Read Foreign Keys
		fks, err := readMSSQLFKs(ctx, r.db, tn)
		if err == nil {
			for name, fk := range fks {
				tbl.Constraints[name] = fk
			}
		}

		sm.Tables[tn] = tbl
	}
	return sm, nil
}

func readMSSQLFKs(ctx context.Context, db *sql.DB, table string) (map[string]*migrate.ConstraintModel, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT 
			obj.name AS constraint_name,
			col1.name AS column_name,
			tab2.name AS referenced_table_name,
			col2.name AS referenced_column_name
		FROM sys.foreign_key_columns fkc
		INNER JOIN sys.objects obj ON obj.object_id = fkc.constraint_object_id
		INNER JOIN sys.tables tab1 ON tab1.object_id = fkc.parent_object_id
		INNER JOIN sys.columns col1 ON col1.column_id = fkc.parent_column_id AND col1.object_id = tab1.object_id
		INNER JOIN sys.tables tab2 ON tab2.object_id = fkc.referenced_object_id
		INNER JOIN sys.columns col2 ON col2.column_id = fkc.referenced_column_id AND col2.object_id = tab2.object_id
		WHERE tab1.name = @p1`, table)
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
