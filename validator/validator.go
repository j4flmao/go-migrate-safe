package validator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/store"
)

// Issue represents a single validation finding.
type Issue struct {
	Code       string
	Message    string
	Table      string
	Column     string
	Suggestion string
	Blocking   bool
	Agent      string
}

// Report is the output of a validation run.
type Report struct {
	Mode       string
	StatusValue string
	Errors      []Issue
	Warnings    []Issue
	ChecksRun   int
	ChecksPass  int
}

func (r *Report) Status() string {
	if r.StatusValue != "" {
		return r.StatusValue
	}
	if len(r.Errors) > 0 {
		return "error"
	}
	if len(r.Warnings) > 0 {
		return "warning"
	}
	return "ok"
}

// Validator checks plans and files for safety/conflicts.
type Validator struct {
	safety migrate.SafetyOptions
}

// New constructs a Validator.
func New(safety migrate.SafetyOptions) *Validator {
	return &Validator{safety: safety}
}

// PreGenerate validates a DiffPlan before codegen.
func (v *Validator) PreGenerate(plan *migrate.DiffPlan) *Report {
	r := &Report{Mode: "pre-generate"}
	if plan == nil || plan.IsEmpty {
		r.StatusValue = "ok"
		return r
	}
	for _, op := range plan.Operations {
		r.ChecksRun++
		switch op.Kind {
		case migrate.OpDropTable:
			if !v.safety.AllowDropTable {
				r.Errors = append(r.Errors, Issue{
					Code: "ErrDestructiveOp", Message: fmt.Sprintf(
						"DROP TABLE %q blocked. Set WithAllowDropTable() to permit this.", op.Table),
					Table: op.Table, Blocking: true, Agent: "validator",
				})
			}
		case migrate.OpDropColumn:
			if !v.safety.AllowDropColumn {
				r.Errors = append(r.Errors, Issue{
					Code: "ErrDestructiveOp", Message: fmt.Sprintf(
						"DROP COLUMN %q.%q blocked. Set WithAllowDropColumn() to permit this.", op.Table, op.Column),
					Table: op.Table, Column: op.Column, Blocking: true, Agent: "validator",
				})
			}
		case migrate.OpAlterColumn:
			if op.Before != nil && op.After != nil && op.Before.SQLType != op.After.SQLType {
				if v.safety.AllowTypeChange == migrate.TypeChangeNone {
					r.Errors = append(r.Errors, Issue{
						Code: "ErrDestructiveOp", Message: fmt.Sprintf(
							"Type change %q -> %q on %q.%q blocked (allow_type_change=none).",
							op.Before.SQLType, op.After.SQLType, op.Table, op.Column),
						Table: op.Table, Column: op.Column, Blocking: true, Agent: "validator",
					})
				} else if v.safety.AllowTypeChange == migrate.TypeChangeSafe && isLossyChange(op.Before.SQLType, op.After.SQLType) {
					r.Errors = append(r.Errors, Issue{
						Code: "ErrDestructiveOp", Message: fmt.Sprintf(
							"Type change %q -> %q on %q.%q may lose data.",
							op.Before.SQLType, op.After.SQLType, op.Table, op.Column),
						Table: op.Table, Column: op.Column, Blocking: true, Agent: "validator",
					})
				}
			}
		}
		if op.Kind == migrate.OpAddColumn && op.After != nil {
			if !op.After.Nullable && op.After.Default == nil {
				r.Warnings = append(r.Warnings, Issue{
					Code: "WarnAddNotNullWithoutDefault",
					Message: fmt.Sprintf("Adding NOT NULL column %q.%q with no DEFAULT. Existing rows will fail if table has data.",
						op.Table, op.Column),
					Table: op.Table, Column: op.Column, Agent: "validator",
					Suggestion: "Add a DEFAULT value or ensure the table is empty before applying.",
				})
			}
		}
		if op.Kind == migrate.OpAlterColumn && op.Before != nil && op.After != nil {
			if op.Before.Nullable && !op.After.Nullable {
				r.Warnings = append(r.Warnings, Issue{
					Code: "WarnNotNullChange",
					Message: fmt.Sprintf("Changing %q.%q from NULL to NOT NULL. Existing code may need updating.",
						op.Table, op.Column),
					Table: op.Table, Column: op.Column, Agent: "validator",
				})
			}
		}
		r.ChecksPass++
	}
	r.StatusValue = r.Status()
	return r
}

// FullValidate checks all migration files in dir.
func (v *Validator) FullValidate(_ context.Context, files []store.File, hist []driver.MigrationRecord) *Report {
	r := &Report{Mode: "full-validate"}
	seen := map[int64][]store.File{}
	for _, f := range files {
		seen[f.Version] = append(seen[f.Version], f)
	}
	versions := make([]int64, 0, len(seen))
	for ver := range seen {
		versions = append(versions, ver)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	for i := 1; i < len(versions); i++ {
		r.ChecksRun++
		if versions[i] != versions[i-1]+1 {
			r.Warnings = append(r.Warnings, Issue{
				Code: "WarnVersionGap",
				Message: fmt.Sprintf("Version gap detected: %d is missing. Was a file deleted?", versions[i-1]+1),
				Agent: "validator",
			})
		}
		r.ChecksPass++
	}
	histMap := map[int64]driver.MigrationRecord{}
	for _, h := range hist {
		if h.Direction == "up" && h.Status == "applied" {
			histMap[h.Version] = h
		}
	}
	for _, f := range files {
		if f.Direction != "up" {
			continue
		}
		r.ChecksRun++
		if h, ok := histMap[f.Version]; ok {
			if h.Checksum != f.Checksum {
				r.Errors = append(r.Errors, Issue{
					Code: "ErrChecksumMismatch",
					Message: fmt.Sprintf("Migration v%d %q was modified after being applied.", f.Version, f.Name),
					Table: f.Name, Blocking: true, Agent: "validator",
					Suggestion: "Revert the file or create a new migration.",
				})
			}
		}
		r.ChecksPass++
	}
	r.StatusValue = r.Status()
	return r
}

// PreApply validates pending migrations before execution.
func (v *Validator) PreApply(_ context.Context, files []store.File, hist []driver.MigrationRecord) *Report {
	r := &Report{Mode: "pre-apply"}
	pending, _ := store.PendingFiles(".", hist)
	_ = files
	for _, f := range pending {
		r.ChecksRun++
		if f.Direction == "up" && f.Checksum == "" {
			r.Errors = append(r.Errors, Issue{
				Code: "ErrMissingDownMigration",
				Message: fmt.Sprintf("Migration v%d %q has no valid checksum.", f.Version, f.Name),
				Blocking: true, Agent: "validator",
			})
		}
		r.ChecksPass++
	}
	r.StatusValue = r.Status()
	return r
}

func isLossyChange(from, to string) bool {
	rank := map[string]int{
		"SMALLINT": 1, "INTEGER": 2, "BIGINT": 3,
		"FLOAT": 1, "DOUBLE": 2,
	}
	if ar, ok := rank[from]; ok {
		if br, ok := rank[to]; ok {
			return br < ar
		}
	}
	if strings.HasPrefix(from, "VARCHAR") && strings.HasPrefix(to, "VARCHAR") {
		return varcharSize(to) < varcharSize(from)
	}
	if from == "TEXT" && strings.HasPrefix(to, "VARCHAR") {
		return true
	}
	return true
}

func varcharSize(s string) int {
	open := strings.IndexByte(s, '(')
	close := strings.IndexByte(s, ')')
	if open <= 0 || close <= open {
		return 0
	}
	n := 0
	for _, r := range s[open+1 : close] {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
