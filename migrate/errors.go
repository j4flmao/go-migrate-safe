package migrate

import "errors"

// Sentinel errors. Callers can use errors.Is to test these.
var (
	// ErrNoModels is returned when Diff() is called with no models registered.
	ErrNoModels = errors.New("no models registered: use WithModels()")

	// ErrMigrationConflict is returned when two pending migrations
	// modify the same column or table.
	ErrMigrationConflict = errors.New("migration conflict detected")

	// ErrDestructiveOp is returned when a destructive operation is in the plan
	// but no opt-in option was provided.
	ErrDestructiveOp = errors.New("destructive operation requires explicit opt-in")

	// ErrChecksumMismatch is returned when an applied migration file has been
	// modified after it was applied.
	ErrChecksumMismatch = errors.New("applied migration file has been modified")

	// ErrLockTimeout is returned when the advisory lock cannot be acquired.
	ErrLockTimeout = errors.New("could not acquire migration lock")

	// ErrMissingDownMigration is returned when Apply() is called but a
	// required .down.sql file is missing or is a stub.
	ErrMissingDownMigration = errors.New("down migration file is required but missing or incomplete")

	// ErrOutOfOrder is returned when a migration version is lower than
	// the latest applied version.
	ErrOutOfOrder = errors.New("migration version is out of order")

	// ErrUnsupportedDriver is returned when the DB driver is not recognized.
	ErrUnsupportedDriver = errors.New("unsupported database driver")

	// ErrInvalidIdentifier is returned when a struct/field name produces
	// a SQL identifier that fails the strict allowlist.
	ErrInvalidIdentifier = errors.New("invalid SQL identifier")

	// ErrInvalidStructTag is returned when a `db:"..."` tag cannot be parsed.
	ErrInvalidStructTag = errors.New("invalid struct tag")
)
