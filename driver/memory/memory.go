// Package memory is an in-memory Driver implementation suitable for tests
// and offline dry-runs.
//
// It keeps an internal SchemaModel that is mutated by Exec(...) — but only
// for a tiny subset of DDL we need for round-trip tests. Real DDL is
// captured verbatim in ExecLog so callers can assert on what was emitted.
package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

// Driver is an in-memory backend.
type Driver struct {
	mu       sync.Mutex
	schema   *migrate.SchemaModel
	History  []driver.MigrationRecord
	ExecLog  []string
	Locked   bool
	HistInit bool
	Version  string
}

// New constructs a memory Driver.
func New() *Driver {
	return &Driver{
		schema:  migrate.NewSchemaModel("public"),
		Version: "memory-1.0",
	}
}

// SeedSchema replaces the internal schema model (for tests).
func (d *Driver) SeedSchema(s *migrate.SchemaModel) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.schema = s
}

func (d *Driver) ReadSchema(_ context.Context, _ string) (*migrate.SchemaModel, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return cloneSchema(d.schema), nil
}

func (d *Driver) Exec(_ context.Context, sql string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ExecLog = append(d.ExecLog, sql)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "INVALID") {
		return fmt.Errorf("memory driver: invalid SQL")
	}
	return nil
}

type memTx struct{ d *Driver }

func (t *memTx) Exec(_ context.Context, sql string) error {
	t.d.ExecLog = append(t.d.ExecLog, sql)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "INVALID") {
		return fmt.Errorf("memory driver: invalid SQL")
	}
	return nil
}

func (d *Driver) ExecTx(_ context.Context, fn func(driver.Tx) error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return fn(&memTx{d})
}

func (d *Driver) AcquireLock(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.Locked {
		return fmt.Errorf("memory driver: %w", migrate.ErrLockTimeout)
	}
	d.Locked = true
	return nil
}

func (d *Driver) ReleaseLock(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Locked = false
	return nil
}

func (d *Driver) EnsureHistoryTable(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.HistInit = true
	return nil
}

func (d *Driver) LoadHistory(_ context.Context) ([]driver.MigrationRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]driver.MigrationRecord, len(d.History))
	copy(out, d.History)
	return out, nil
}

func (d *Driver) RecordMigration(_ context.Context, r driver.MigrationRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.History = append(d.History, r)
	return nil
}

func (d *Driver) CheckNulls(_ context.Context, _, _ string) (int64, error) {
	return 0, nil
}

func (d *Driver) DriverName() string                                { return "memory" }
func (d *Driver) DatabaseVersion(_ context.Context) (string, error) { return d.Version, nil }
func (d *Driver) Close() error                                      { return nil }

func cloneSchema(s *migrate.SchemaModel) *migrate.SchemaModel {
	if s == nil {
		return migrate.NewSchemaModel("")
	}
	c := migrate.NewSchemaModel(s.Schema)
	for tn, t := range s.Tables {
		ct := migrate.NewTableModel(t.Name)
		ct.ColumnOrder = append(ct.ColumnOrder, t.ColumnOrder...)
		for cn, col := range t.Columns {
			cc := *col
			ct.Columns[cn] = &cc
		}
		for in, idx := range t.Indexes {
			ci := *idx
			ci.Columns = append([]string{}, idx.Columns...)
			ct.Indexes[in] = &ci
		}
		for kn, k := range t.Constraints {
			cc := *k
			cc.Columns = append([]string{}, k.Columns...)
			cc.RefColumns = append([]string{}, k.RefColumns...)
			ct.Constraints[kn] = &cc
		}
		c.Tables[tn] = ct
	}
	return c
}
