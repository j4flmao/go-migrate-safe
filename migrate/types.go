// Package migrate is the main public entry point for the go-migrate-safe library.
//
// It exposes the Migrator type, the DiffPlan it produces, and the functional
// Option set used to configure it. Users typically only import this package
// plus exactly one driver package (migrate/driver/<name>).
package migrate

import "context"

// SchemaModel is a database-agnostic representation of a database schema.
// It is produced by both the Struct Parser (from Go structs) and the
// DB Schema Reader (from a live database) so that the Diff Engine can
// compare them with one comparator.
type SchemaModel struct {
	// Schema is the logical schema name (e.g. "public" on Postgres).
	Schema string
	// Tables is keyed by table name.
	Tables map[string]*TableModel
}

// NewSchemaModel constructs an empty SchemaModel ready for population.
func NewSchemaModel(schema string) *SchemaModel {
	return &SchemaModel{Schema: schema, Tables: map[string]*TableModel{}}
}

// TableModel describes a single table.
type TableModel struct {
	Name        string
	Columns     map[string]*ColumnModel
	Indexes     map[string]*IndexModel
	Constraints map[string]*ConstraintModel
	// ColumnOrder preserves the declaration order of columns (parser only).
	// The DB reader fills it from ordinal_position.
	ColumnOrder []string
}

// NewTableModel constructs an empty TableModel.
func NewTableModel(name string) *TableModel {
	return &TableModel{
		Name:        name,
		Columns:     map[string]*ColumnModel{},
		Indexes:     map[string]*IndexModel{},
		Constraints: map[string]*ConstraintModel{},
	}
}

// ColumnModel describes a single column in normalized form.
type ColumnModel struct {
	Name          string
	SQLType       string  // canonical normalized form, e.g. "BIGINT", "VARCHAR(50)"
	Nullable      bool
	Default       *string // nil => no default; pointer to string => "NOW()", "'pending'", etc.
	IsPK          bool
	AutoIncrement bool
	Size          *int // for VARCHAR(n)
	Precision     *int // for DECIMAL(p,s)
	Scale         *int
}

// IndexModel describes a single index.
type IndexModel struct {
	Name    string
	Columns []string
	Unique  bool
}

// ConstraintKind enumerates the kinds of constraints we model.
type ConstraintKind string

const (
	ConstraintPrimaryKey ConstraintKind = "pk"
	ConstraintForeignKey ConstraintKind = "fk"
	ConstraintUnique     ConstraintKind = "unique"
	ConstraintCheck      ConstraintKind = "check"
)

// ConstraintModel describes a single constraint.
type ConstraintModel struct {
	Name       string
	Kind       ConstraintKind
	Columns    []string
	RefTable   string   // FK only
	RefColumns []string // FK only
	CheckExpr  string   // CHECK only
}

// MigrationContext carries shared invocation context to all internal agents.
type MigrationContext struct {
	LibraryVersion string
	GoVersion      string
	DBDriver       string // "postgres" | "mysql" | "sqlite"
	DBVersion      string
	Schema         string
	OutputDir      string
	SafetyOpts     SafetyOptions
	NextVersion    int64
	ExistingVers   []int64
}

// SafetyOptions controls which destructive operations are permitted.
type SafetyOptions struct {
	AllowDropTable    bool
	AllowDropColumn   bool
	AllowTypeChange   TypeChangeMode
	NoRollbackRequired bool
	AutoBackfillStep  bool
}

// TypeChangeMode controls how lossy ALTER COLUMN type changes are handled.
type TypeChangeMode string

const (
	// TypeChangeNone disallows any type change. (Validator errors on any.)
	TypeChangeNone TypeChangeMode = "none"
	// TypeChangeSafe allows only widening (non-lossy) changes.
	TypeChangeSafe TypeChangeMode = "safe"
	// TypeChangeAny allows all type changes including lossy ones.
	TypeChangeAny TypeChangeMode = "any"
)

// MigrationRecord is one row in the _migrate_history table.
type MigrationRecord struct {
	Version      int64
	Name         string
	Direction    string // "up" | "down"
	Checksum     string // sha256 hex (no prefix)
	AppliedAt    string // RFC3339
	ExecutionMS  int64
	Status       string // "applied" | "failed" | "rolled_back"
	ErrorMessage string
}

// Tx is a minimal abstraction over a database transaction.
type Tx interface {
	Exec(ctx context.Context, sql string) error
}

// Driver is the contract every database backend must satisfy.
type Driver interface {
	// Schema introspection
	ReadSchema(ctx context.Context, schema string) (*SchemaModel, error)

	// Execution
	Exec(ctx context.Context, sql string) error
	ExecTx(ctx context.Context, fn func(tx Tx) error) error

	// Locking
	AcquireLock(ctx context.Context) error
	ReleaseLock(ctx context.Context) error

	// State store
	EnsureHistoryTable(ctx context.Context) error
	LoadHistory(ctx context.Context) ([]MigrationRecord, error)
	RecordMigration(ctx context.Context, r MigrationRecord) error

	// Introspection
	DriverName() string
	DatabaseVersion(ctx context.Context) (string, error)

	// Close releases all resources.
	Close() error
}
