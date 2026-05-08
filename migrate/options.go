package migrate

import (
	"database/sql"
	"log/slog"
)

// Option configures a Migrator. Options use the functional pattern so the
// public API can grow without breaking existing call sites.
type Option func(*config)

type config struct {
	models        []any
	outputDir     string
	schema        string
	driverName    string
	logger        *slog.Logger
	safetyOpts    SafetyOptions
	versionStyle  string // "sequential" | "timestamp"
	renameColumns []renameColumnSpec
	db            *sql.DB
	dbDriver      Driver
}

type renameColumnSpec struct {
	Table   string
	OldName string
	NewName string
}

func defaultConfig() *config {
	return &config{
		outputDir:    "./migrations",
		schema:       "public",
		versionStyle: "sequential",
		safetyOpts: SafetyOptions{
			AllowTypeChange: TypeChangeNone,
		},
	}
}

// WithModels registers Go structs as the desired schema source of truth.
func WithModels(models ...any) Option {
	return func(c *config) { c.models = append(c.models, models...) }
}

// WithOutputDir sets the directory where migration files are written.
func WithOutputDir(dir string) Option {
	return func(c *config) { c.outputDir = dir }
}

// WithSchema sets the target schema name (e.g. "public").
func WithSchema(s string) Option {
	return func(c *config) { c.schema = s }
}

// WithDriver selects a SQL dialect: "postgres" | "mysql" | "sqlite".
func WithDriver(name string) Option {
	return func(c *config) { c.driverName = name }
}

// WithLogger injects a structured logger for the library.
func WithLogger(l *slog.Logger) Option {
	return func(c *config) { c.logger = l }
}

// WithAllowDropTable opts in to permit DROP TABLE operations.
func WithAllowDropTable() Option {
	return func(c *config) { c.safetyOpts.AllowDropTable = true }
}

// WithAllowDropColumn opts in to permit DROP COLUMN operations.
func WithAllowDropColumn() Option {
	return func(c *config) { c.safetyOpts.AllowDropColumn = true }
}

// WithAllowTypeChange permits ALTER COLUMN type changes at the requested level.
func WithAllowTypeChange(mode TypeChangeMode) Option {
	return func(c *config) { c.safetyOpts.AllowTypeChange = mode }
}

// WithNoRollbackRequired disables the missing-down-migration check.
// Not recommended for production.
func WithNoRollbackRequired() Option {
	return func(c *config) { c.safetyOpts.NoRollbackRequired = true }
}

// WithAutoBackfillStep auto-generates a two-step migration when an
// ADD COLUMN NOT NULL without DEFAULT is detected.
func WithAutoBackfillStep() Option {
	return func(c *config) { c.safetyOpts.AutoBackfillStep = true }
}

// WithRenameColumn declares an explicit rename so the diff engine emits
// ALTER TABLE ... RENAME COLUMN instead of DROP+ADD.
func WithRenameColumn(table, oldName, newName string) Option {
	return func(c *config) {
		c.renameColumns = append(c.renameColumns, renameColumnSpec{table, oldName, newName})
	}
}

// WithVersionStyle sets the version-numbering style: "sequential" or "timestamp".
func WithVersionStyle(style string) Option {
	return func(c *config) { c.versionStyle = style }
}

// WithDB sets the database connection to use.
// Required for Apply, Status, History, Rollback, Validate.
func WithDB(db *sql.DB) Option {
	return func(c *config) { c.db = db }
}

// WithDriverInstance sets the DB driver instance explicitly.
// Overrides the driver that would be auto-detected from WithDB.
func WithDriverInstance(drv Driver) Option {
	return func(c *config) { c.dbDriver = drv }
}

// DriverInstance returns the configured Driver, or nil.
func (m *Migrator) DriverInstance() Driver { return m.cfg.dbDriver }
