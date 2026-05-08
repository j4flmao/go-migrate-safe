package mssql

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
	reader *reader.MSSQLReader
}

func New(db *sql.DB) *Driver {
	return &Driver{db: db, reader: reader.NewMSSQL(db)}
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
	if err := fn(&mssqlTx{tx: tx}); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type mssqlTx struct{ tx *sql.Tx }

func (t *mssqlTx) Exec(ctx context.Context, sql string) error {
	_, err := t.tx.ExecContext(ctx, sql)
	return err
}

func (d *Driver) AcquireLock(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "sp_getapplock 'migrate_lock', 'Exclusive', 'Session', 10000")
	return err
}

func (d *Driver) ReleaseLock(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, "sp_releaseapplock 'migrate_lock', 'Session'")
	return err
}

func (d *Driver) EnsureHistoryTable(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `
		IF OBJECT_ID('dbo._migrate_history', 'U') IS NULL
		CREATE TABLE _migrate_history (
			version BIGINT PRIMARY KEY,
			name NVARCHAR(255) NOT NULL,
			direction NVARCHAR(4) NOT NULL,
			checksum NVARCHAR(255) NOT NULL,
			applied_at NVARCHAR(255) NOT NULL,
			execution_ms BIGINT NOT NULL DEFAULT 0,
			status NVARCHAR(16) NOT NULL DEFAULT 'applied',
			error_message NVARCHAR(MAX)
		)`)
	return err
}

func (d *Driver) LoadHistory(ctx context.Context) ([]migrate.MigrationRecord, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT version, name, direction, checksum, applied_at, execution_ms, status, ISNULL(error_message,'') FROM _migrate_history ORDER BY version`)
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
		 VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8)`,
		r.Version, r.Name, r.Direction, r.Checksum, r.AppliedAt, r.ExecutionMS, r.Status, r.ErrorMessage)
	return err
}

func (d *Driver) DriverName() string { return "mssql" }

func (d *Driver) DatabaseVersion(ctx context.Context) (string, error) {
	var ver string
	err := d.db.QueryRowContext(ctx, "SELECT @@VERSION").Scan(&ver)
	return ver, err
}

func (d *Driver) Close() error {
	return d.db.Close()
}

var _ migrate.Driver = (*Driver)(nil)
