package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
)

// Version is the library version reported in generated file headers.
const Version = "0.1.0"

// Migrator is the main entry point. Create one per application.
// It is safe for concurrent use after construction.
type Migrator struct {
	cfg *config
}

// New constructs a Migrator with the given options.
func New(opts ...Option) (*Migrator, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.logger == nil {
		cfg.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.driverName == "" {
		cfg.driverName = "postgres"
	}
	return &Migrator{cfg: cfg}, nil
}

// Models returns the registered model values.
func (m *Migrator) Models() []any { return m.cfg.models }

// OutputDir returns the configured output directory.
func (m *Migrator) OutputDir() string { return m.cfg.outputDir }

// Schema returns the target schema name.
func (m *Migrator) Schema() string { return m.cfg.schema }

// Driver returns the configured driver name.
func (m *Migrator) Driver() string { return m.cfg.driverName }

// Logger returns the structured logger.
func (m *Migrator) Logger() *slog.Logger { return m.cfg.logger }

// SafetyOptions returns a copy of the active safety options.
func (m *Migrator) SafetyOptions() SafetyOptions { return m.cfg.safetyOpts }

// RenameSpecs returns the explicit column-rename declarations.
func (m *Migrator) RenameSpecs() []RenameSpec {
	out := make([]RenameSpec, 0, len(m.cfg.renameColumns))
	for _, r := range m.cfg.renameColumns {
		out = append(out, RenameSpec{Table: r.Table, OldName: r.OldName, NewName: r.NewName})
	}
	return out
}

// RenameSpec is the public, exported form of a rename declaration.
type RenameSpec struct {
	Table   string
	OldName string
	NewName string
}

// Context builds a MigrationContext snapshot for use by internal agents.
func (m *Migrator) Context(nextVersion int64, existing []int64) MigrationContext {
	return MigrationContext{
		LibraryVersion: Version,
		DBDriver:       m.cfg.driverName,
		Schema:         m.cfg.schema,
		OutputDir:      m.cfg.outputDir,
		SafetyOpts:     m.cfg.safetyOpts,
		NextVersion:    nextVersion,
		ExistingVers:   existing,
	}
}

func (m *Migrator) driverOrError() (Driver, error) {
	if m.cfg.dbDriver != nil {
		return m.cfg.dbDriver, nil
	}
	return nil, fmt.Errorf("migrate: %w: no driver configured; use WithDB or WithDriverInstance", errNoDriver)
}

// ShadowDriver is a special driver that can be used for shadow database validation.
type ShadowDriver interface {
	Driver
	Reset(ctx context.Context) error // Reset the shadow database to an empty state
}

// Diff compares the registered struct models against the current DB schema
// and returns a DiffPlan describing what needs to change.
// Diff does NOT modify the database or generate any files.
func (m *Migrator) Diff(ctx context.Context) (*DiffPlan, error) {
	// Implement actual diff logic using orchestrator.Run internally or similar
	return nil, fmt.Errorf("migrate.Diff: %w", errNotImplemented)
}

// Apply generates migration files (if not already generated) and applies all
// pending migrations to the database.
func (m *Migrator) Apply(ctx context.Context, opts ...ApplyOption) error {
	drv, err := m.driverOrError()
	if err != nil {
		return err
	}
	_ = drv
	_ = opts
	return fmt.Errorf("migrate.Apply: %w", errNotImplemented)
}

// Status returns the current migration state.
func (m *Migrator) Status(ctx context.Context) (*StatusReport, error) {
	drv, err := m.driverOrError()
	if err != nil {
		return nil, err
	}
	hist, err := drv.LoadHistory(ctx)
	if err != nil {
		return nil, fmt.Errorf("migrate.Status: %w", err)
	}
	report := &StatusReport{}
	for _, r := range hist {
		report.Applied = append(report.Applied, r)
	}
	return report, nil
}

// History returns the full migration history from the state store.
func (m *Migrator) History(ctx context.Context) ([]MigrationRecord, error) {
	drv, err := m.driverOrError()
	if err != nil {
		return nil, err
	}
	return drv.LoadHistory(ctx)
}

// Rollback rolls back the last N applied migrations. N defaults to 1.
func (m *Migrator) Rollback(ctx context.Context, opts ...RollbackOption) error {
	drv, err := m.driverOrError()
	if err != nil {
		return err
	}
	_ = drv
	_ = opts
	return fmt.Errorf("migrate.Rollback: %w", errNotImplemented)
}

// Validate checks all migration files in the output directory for conflicts,
// missing down migrations, checksum drift, and ordering issues.
func (m *Migrator) Validate(ctx context.Context) (*ValidationReport, error) {
	drv, err := m.driverOrError()
	if err != nil {
		return nil, err
	}
	hist, err := drv.LoadHistory(ctx)
	if err != nil {
		hist = nil
	}
	_ = hist
	return nil, fmt.Errorf("migrate.Validate: %w", errNotImplemented)
}

// HasDB reports whether a database connection or driver has been configured.
func (m *Migrator) HasDB() bool {
	return m.cfg.db != nil || m.cfg.dbDriver != nil
}

// DB returns the configured *sql.DB, may be nil.
func (m *Migrator) DB() *sql.DB { return m.cfg.db }

// errNoDriver is returned when a DB operation is attempted without a driver.
var errNoDriver = fmt.Errorf("no database driver available")

var errNotImplemented = fmt.Errorf("not implemented yet")
