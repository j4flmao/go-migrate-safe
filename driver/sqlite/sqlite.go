package sqlite

import (
	"context"
	"database/sql"
	"sync"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/reader"
)

type Driver struct {
	db     *sql.DB
	mu     sync.Mutex
	reader *reader.SQLiteReader
}

func New(db *sql.DB) *Driver {
	return &Driver{db: db, reader: reader.NewSQLite(db)}
}

func (d *Driver) ReadSchema(ctx context.Context, schema string) (*migrate.SchemaModel, error) {
	return d.reader.ReadSchema(ctx, schema)
}

func (d *Driver) Exec(ctx context.Context, sql string) error {
	_, err := d.db.ExecContext(ctx, sql)
	return err
}

func (d *Driver) ExecTx(ctx context.Context, fn func(tx migrate.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(&sqliteTx{tx: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type sqliteTx struct{ tx *sql.Tx }

func (t *sqliteTx) Exec(ctx context.Context, sql string) error {
	_, err := t.tx.ExecContext(ctx, sql)
	return err
}

func (d *Driver) AcquireLock(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "BEGIN IMMEDIATE")
	return err
}

func (d *Driver) ReleaseLock(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "COMMIT")
	return err
}

func (d *Driver) EnsureHistoryTable(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS _migrate_history (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			direction TEXT NOT NULL,
			checksum TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now')),
			execution_ms INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'applied',
			error_message TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
}

func (d *Driver) LoadHistory(ctx context.Context) ([]migrate.MigrationRecord, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT version, name, direction, checksum, applied_at, execution_ms, status, error_message FROM _migrate_history ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []migrate.MigrationRecord
	for rows.Next() {
		var r migrate.MigrationRecord
		if err := rows.Scan(&r.Version, &r.Name, &r.Direction, &r.Checksum, &r.AppliedAt, &r.ExecutionMS, &r.Status, &r.ErrorMessage); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (d *Driver) RecordMigration(ctx context.Context, r migrate.MigrationRecord) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO _migrate_history (version, name, direction, checksum, applied_at, execution_ms, status, error_message)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Version, r.Name, r.Direction, r.Checksum, r.AppliedAt, r.ExecutionMS, r.Status, r.ErrorMessage)
	return err
}

func (d *Driver) DriverName() string { return "sqlite" }

func (d *Driver) DatabaseVersion(ctx context.Context) (string, error) {
	var ver string
	err := d.db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&ver)
	return ver, err
}

func (d *Driver) CheckNulls(ctx context.Context, table, column string) (int64, error) {
	var count int64
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + column + " IS NULL"
	err := d.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (d *Driver) Close() error {
	return d.db.Close()
}

var _ migrate.Driver = (*Driver)(nil)
