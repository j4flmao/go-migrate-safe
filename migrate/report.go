package migrate

import "time"

// StatusReport describes the current migration state of the DB.
type StatusReport struct {
	Applied []MigrationRecord
	Pending []PendingMigration
	Dirty   []DirtyMigration
}

// PendingMigration describes a migration file not yet applied.
type PendingMigration struct {
	Version int64
	Name    string
	UpFile  string
}

// DirtyMigration describes an applied migration whose checksum no longer matches.
type DirtyMigration struct {
	Version     int64
	Name        string
	ExpectedSum string
	ActualSum   string
}

// ValidationReport is the result of a Validate() call.
type ValidationReport struct {
	StatusValue string
	Errors      []string
	Warnings    []string
}

// ApplyOption configures an Apply call.
type ApplyOption func(*applyConfig)

// RollbackOption configures a Rollback call.
type RollbackOption func(*rollbackConfig)

type applyConfig struct {
	timeout     time.Duration
	appliedBy   string
	stepTimeout time.Duration
}

type rollbackConfig struct {
	n         int
	toVersion int64
}

// WithTimeout sets the maximum duration for the entire Apply operation.
func WithTimeout(d time.Duration) ApplyOption {
	return func(c *applyConfig) { c.timeout = d }
}

// WithAppliedBy sets the "applied_by" field in migration history.
func WithAppliedBy(name string) ApplyOption {
	return func(c *applyConfig) { c.appliedBy = name }
}

// WithStepTimeout sets the timeout per individual migration step.
func WithStepTimeout(d time.Duration) ApplyOption {
	return func(c *applyConfig) { c.stepTimeout = d }
}

// WithRollbackN sets the number of migrations to roll back.
func WithRollbackN(n int) RollbackOption {
	return func(c *rollbackConfig) { c.n = n }
}

// WithRollbackTo rolls back to a specific version.
func WithRollbackTo(version int64) RollbackOption {
	return func(c *rollbackConfig) { c.toVersion = version }
}

// DryRunReport describes the outcome of a dry-run execution.
type DryRunReport struct {
	Success      bool
	StepsApplied int
	StepsFailed  int
	DurationMs   int64
	Steps        []DryRunStep
	Errors       []error
}

// DryRunStep describes one migration step in a dry-run.
type DryRunStep struct {
	Version    int64
	Name       string
	SQL        string
	DurationMs int64
	Error      error
}
