package migrate

import (
	"context"
	"fmt"
	"strings"
)

// OpKind enumerates the kinds of schema change operations.
type OpKind string

const (
	OpAddTable        OpKind = "add_table"
	OpDropTable       OpKind = "drop_table"
	OpAddColumn       OpKind = "add_column"
	OpDropColumn      OpKind = "drop_column"
	OpAlterColumn     OpKind = "alter_column"
	OpRenameColumn    OpKind = "rename_column"
	OpAddIndex        OpKind = "add_index"
	OpDropIndex       OpKind = "drop_index"
	OpAddConstraint   OpKind = "add_constraint"
	OpDropConstraint  OpKind = "drop_constraint"
)

// Operation is a single schema change.
type Operation struct {
	Kind     OpKind
	Table    string
	Column   string // empty for table-level ops
	Index    string // empty for non-index ops

	Before *ColumnModel // pre-change state (alter)
	After  *ColumnModel // post-change state (alter)

	// NewTable is the full table model for OpAddTable.
	NewTable *TableModel
	// IndexDef is set for OpAddIndex/OpDropIndex.
	IndexDef *IndexModel
	// ConstraintDef is set for OpAddConstraint/OpDropConstraint.
	ConstraintDef *ConstraintModel

	SQL      string // rendered SQL (filled by codegen)
	IsUnsafe bool   // true if op can cause data loss
	Reason   string // human-readable explanation
}

// RenameHint suggests that a drop+add pair may actually be a rename.
type RenameHint struct {
	Table          string
	DroppedColumn  string
	AddedColumn    string
	Confidence     string // "high" | "medium" | "low"
	Reason         string
}

// Warning is a non-blocking issue surfaced to the user.
type Warning struct {
	Code       string
	Message    string
	Table      string
	Column     string
	Suggestion string
}

// DiffPlan describes the operations needed to bring the DB in sync
// with the defined struct models.
type DiffPlan struct {
	Version           int64
	Name              string
	Operations        []Operation
	RollbackOps       []Operation
	HasDestructiveOps bool
	Warnings          []Warning
	GeneratedFiles    []string
	RenameHints       []RenameHint
	IsEmpty           bool
}

// Generate writes the migration files to disk.
// Returns the paths of the generated files.
func (p *DiffPlan) Generate() ([]string, error) {
	return nil, fmt.Errorf("DiffPlan.Generate: %w", errNotImplemented)
}

// Apply applies the migration plan to the database.
func (p *DiffPlan) Apply(ctx context.Context) error {
	return fmt.Errorf("DiffPlan.Apply: %w", errNotImplemented)
}

// DryRun applies the plan to a temporary schema and rolls back,
// verifying the SQL is valid without modifying production data.
func (p *DiffPlan) DryRun(ctx context.Context) (*DryRunReport, error) {
	return nil, fmt.Errorf("DiffPlan.DryRun: %w", errNotImplemented)
}

// Explain returns a human-readable summary of the plan.
func (p *DiffPlan) Explain() string {
	if p.IsEmpty || len(p.Operations) == 0 {
		return "No schema changes detected. Nothing to generate.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Migration v%04d — %s\n", p.Version, p.Name)
	fmt.Fprintf(&b, "Operations (%d):\n", len(p.Operations))
	for i, op := range p.Operations {
		marker := " "
		if op.IsUnsafe {
			marker = "⚠"
		}
		fmt.Fprintf(&b, "  %s [%d/%d] %s — %s\n", marker, i+1, len(p.Operations), op.Kind, op.Reason)
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintf(&b, "Warnings (%d):\n", len(p.Warnings))
		for _, w := range p.Warnings {
			fmt.Fprintf(&b, "  - [%s] %s\n", w.Code, w.Message)
		}
	}
	if len(p.RenameHints) > 0 {
		fmt.Fprintf(&b, "Possible renames:\n")
		for _, h := range p.RenameHints {
			fmt.Fprintf(&b, "  - %s.%s → %s.%s (%s confidence)\n",
				h.Table, h.DroppedColumn, h.Table, h.AddedColumn, h.Confidence)
		}
	}
	return b.String()
}
