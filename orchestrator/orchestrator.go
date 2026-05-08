// Package orchestrator wires the diff → validate → codegen → rollback pipeline
// (and the apply / rollback / status pipelines) into a single coordinated flow.
package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/j4flmao/go-migrate-safe/codegen"
	"github.com/j4flmao/go-migrate-safe/diff"
	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/executor"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/parser"
	"github.com/j4flmao/go-migrate-safe/rollback"
	"github.com/j4flmao/go-migrate-safe/store"
	"github.com/j4flmao/go-migrate-safe/validator"
)

// Step is a record of one agent invocation in a pipeline run.
type Step struct {
	Agent       string
	Status      string
	DurationMS  int64
	Summary     string
}

// Result is the outcome of a pipeline run.
type Result struct {
	Pipeline         string // "ok" | "warning" | "error" | "aborted"
	Intent           string
	Steps            []Step
	DiffPlan         *migrate.DiffPlan
	ValidationReport *validator.Report
	RollbackPlan     *rollback.Plan
	GeneratedFiles   []string
	Warnings         []validator.Issue
	Errors           []validator.Issue
	Explain          string
}

// Options configures a single pipeline run.
type Options struct {
	Migrator     *migrate.Migrator
	DBDriver     driver.Driver
	Models       []any
	OutputDir    string
	DialectName  string
	NameOverride string
}

// Run dispatches to the pipeline named by intent.
func Run(ctx context.Context, intent string, opts Options) (*Result, error) {
	switch intent {
	case "diff":
		return runDiff(ctx, opts)
	case "generate":
		return runGenerate(ctx, opts)
	case "validate":
		return runValidate(ctx, opts)
	case "apply":
		return runApply(ctx, opts, false)
	case "apply-dry-run":
		return runApply(ctx, opts, true)
	case "rollback":
		return runRollback(ctx, opts)
	case "status":
		return runStatus(ctx, opts)
	case "history":
		return runHistory(ctx, opts)
	}
	return nil, fmt.Errorf("orchestrator: unknown intent %q", intent)
}

func runDiff(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{Intent: "diff"}
	plan, _, err := computeDiff(ctx, opts)
	if err != nil {
		return res, err
	}
	res.DiffPlan = plan
	res.Explain = plan.Explain()
	res.Pipeline = "ok"
	return res, nil
}

func runGenerate(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{Intent: "generate"}
	plan, dialect, err := computeDiff(ctx, opts)
	if err != nil {
		return res, err
	}
	res.DiffPlan = plan
	if plan.IsEmpty {
		res.Explain = plan.Explain()
		res.Pipeline = "ok"
		return res, nil
	}
	if opts.NameOverride != "" {
		plan.Name = opts.NameOverride
	}

	// Step 2: validate
	step := Step{Agent: "validator"}
	t0 := time.Now()
	report := validator.New(opts.Migrator.SafetyOptions()).PreGenerate(plan)
	step.Status = report.Status()
	step.DurationMS = time.Since(t0).Milliseconds()
	step.Summary = fmt.Sprintf("%d errors, %d warnings", len(report.Errors), len(report.Warnings))
	res.ValidationReport = report
	res.Steps = append(res.Steps, step)
	res.Warnings = append(res.Warnings, report.Warnings...)
	if len(report.Errors) > 0 {
		res.Errors = append(res.Errors, report.Errors...)
		res.Pipeline = "error"
		return res, nil
	}

	// Step 3: codegen
	step = Step{Agent: "codegen"}
	t0 = time.Now()
	g := codegen.New(dialect, opts.OutputDir)
	out, err := g.Generate(plan)
	if err != nil {
		return res, fmt.Errorf("codegen: %w", err)
	}
	step.Status = "ok"
	step.DurationMS = time.Since(t0).Milliseconds()
	step.Summary = fmt.Sprintf("wrote %s, %s", out.UpFile, out.DownFile)
	res.Steps = append(res.Steps, step)
	res.GeneratedFiles = []string{out.UpFile, out.DownFile}

	// Step 4: rollback
	step = Step{Agent: "rollback"}
	t0 = time.Now()
	rp, err := rollback.New(dialect).Build(plan, out.DownFile)
	if err != nil {
		return res, fmt.Errorf("rollback: %w", err)
	}
	step.Status = "ok"
	if rp.RequiresManual {
		step.Status = "warning"
	}
	step.DurationMS = time.Since(t0).Milliseconds()
	step.Summary = fmt.Sprintf("%d rollback ops; manual=%v", len(rp.RollbackOps), rp.RequiresManual)
	res.RollbackPlan = rp
	res.Steps = append(res.Steps, step)

	res.Explain = plan.Explain()
	res.Pipeline = pipelineStatus(res)
	return res, nil
}

func runValidate(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{Intent: "validate"}
	files, err := store.ListFiles(opts.OutputDir)
	if err != nil {
		return res, err
	}
	hist, err := opts.DBDriver.LoadHistory(ctx)
	if err != nil {
		// History may not exist yet; treat as empty.
		hist = nil
	}
	report := validator.New(opts.Migrator.SafetyOptions()).FullValidate(ctx, files, hist)
	res.ValidationReport = report
	res.Errors = append(res.Errors, report.Errors...)
	res.Warnings = append(res.Warnings, report.Warnings...)
	res.Steps = append(res.Steps, Step{
		Agent: "validator", Status: report.Status(),
		Summary: fmt.Sprintf("%d errors, %d warnings", len(report.Errors), len(report.Warnings)),
	})
	res.Pipeline = report.Status()
	return res, nil
}

func runApply(ctx context.Context, opts Options, dryRun bool) (*Result, error) {
	intent := "apply"
	if dryRun {
		intent = "apply-dry-run"
	}
	res := &Result{Intent: intent}
	files, err := store.ListFiles(opts.OutputDir)
	if err != nil {
		return res, err
	}
	hist, _ := opts.DBDriver.LoadHistory(ctx)
	report := validator.New(opts.Migrator.SafetyOptions()).PreApply(ctx, files, hist)
	res.ValidationReport = report
	res.Warnings = append(res.Warnings, report.Warnings...)
	if len(report.Errors) > 0 {
		res.Errors = append(res.Errors, report.Errors...)
		res.Pipeline = "error"
		return res, nil
	}

	pending, err := store.PendingFiles(opts.OutputDir, hist)
	if err != nil {
		return res, err
	}
	if len(pending) == 0 {
		res.Explain = "No pending migrations.\n"
		res.Pipeline = "ok"
		return res, nil
	}

	ex := executor.New(opts.DBDriver)
	ex.DryRun = dryRun
	t0 := time.Now()
	apr, err := ex.Apply(ctx, pending)
	res.Steps = append(res.Steps, Step{
		Agent: "executor",
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		DurationMS: time.Since(t0).Milliseconds(),
		Summary:    fmt.Sprintf("applied=%d failed=%v", len(apr.Applied), apr.Failed != nil),
	})
	if err != nil {
		res.Errors = append(res.Errors, validator.Issue{
			Code: "ErrApplyFailed", Message: err.Error(), Blocking: true, Agent: "executor",
		})
		res.Pipeline = "error"
		return res, err
	}
	var b strings.Builder
	for _, f := range apr.Applied {
		fmt.Fprintf(&b, "Applied v%04d %s\n", f.Version, f.Name)
	}
	res.Explain = b.String()
	res.Pipeline = "ok"
	return res, nil
}

// RollbackOptions parameterises rollback runs.
type RollbackOptions struct {
	N         int
	ToVersion int64
}

func runRollback(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{Intent: "rollback"}
	files, err := store.ListFiles(opts.OutputDir)
	if err != nil {
		return res, err
	}
	// Default rollback: last 1
	hist, _ := opts.DBDriver.LoadHistory(ctx)
	var appliedUp []driver.MigrationRecord
	for _, h := range hist {
		if h.Direction == "up" && h.Status == "applied" {
			appliedUp = append(appliedUp, h)
		}
	}
	n := 1
	if len(appliedUp) < n {
		n = len(appliedUp)
	}
	if n == 0 {
		res.Explain = "Nothing to rollback.\n"
		res.Pipeline = "ok"
		return res, nil
	}
	// Pick the N most-recently-applied versions.
	target := appliedUp[len(appliedUp)-n:]
	var downs []store.File
	for _, t := range target {
		f, err := store.FindFile(opts.OutputDir, t.Version, "down")
		if err != nil {
			res.Errors = append(res.Errors, validator.Issue{
				Code: "ErrMissingDownMigration", Message: err.Error(), Blocking: true, Agent: "executor",
			})
			res.Pipeline = "error"
			return res, nil
		}
		downs = append(downs, *f)
	}
	_ = files
	t0 := time.Now()
	apr, err := executor.New(opts.DBDriver).Rollback(ctx, downs)
	res.Steps = append(res.Steps, Step{
		Agent: "executor",
		Status: func() string {
			if err != nil {
				return "error"
			}
			return "ok"
		}(),
		DurationMS: time.Since(t0).Milliseconds(),
		Summary:    fmt.Sprintf("rolled_back=%d", len(apr.Applied)),
	})
	if err != nil {
		res.Pipeline = "error"
		return res, err
	}
	var b strings.Builder
	for _, f := range apr.Applied {
		fmt.Fprintf(&b, "Rolled back v%04d %s\n", f.Version, f.Name)
	}
	res.Explain = b.String()
	res.Pipeline = "ok"
	return res, nil
}

func runStatus(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{Intent: "status"}
	hist, _ := opts.DBDriver.LoadHistory(ctx)
	statuses, err := store.BuildStatus(ctx, opts.OutputDir, hist)
	if err != nil {
		return res, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Version  Status   Name\n")
	fmt.Fprintf(&b, "-------  -------  ----\n")
	for _, s := range statuses {
		st := "pending"
		if s.Applied {
			st = "applied"
		}
		fmt.Fprintf(&b, "%-7d  %-7s  %s\n", s.Version, st, s.Name)
	}
	if len(statuses) == 0 {
		b.WriteString("(no migrations)\n")
	}
	res.Explain = b.String()
	res.Pipeline = "ok"
	return res, nil
}

func runHistory(ctx context.Context, opts Options) (*Result, error) {
	res := &Result{Intent: "history"}
	hist, err := opts.DBDriver.LoadHistory(ctx)
	if err != nil {
		return res, err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Version  Direction  Status        Name        Applied At\n")
	fmt.Fprintf(&b, "-------  ---------  ------------  ----------  -------------------------\n")
	for _, h := range hist {
		fmt.Fprintf(&b, "%-7d  %-9s  %-12s  %-10s  %s\n", h.Version, h.Direction, h.Status, h.Name, h.AppliedAt)
	}
	if len(hist) == 0 {
		b.WriteString("(no history)\n")
	}
	res.Explain = b.String()
	res.Pipeline = "ok"
	return res, nil
}

// computeDiff is shared by diff and generate.
func computeDiff(ctx context.Context, opts Options) (*migrate.DiffPlan, string, error) {
	dialect := opts.DialectName
	if dialect == "" && opts.Migrator != nil {
		dialect = opts.Migrator.Driver()
	}
	if dialect == "" {
		dialect = "postgres"
	}
	parserDialect := parser.Dialect(dialect)
	schema := "public"
	if opts.Migrator != nil {
		schema = opts.Migrator.Schema()
	}
	models := opts.Models
	if len(models) == 0 && opts.Migrator != nil {
		models = opts.Migrator.Models()
	}
	var want *migrate.SchemaModel
	var pErr error
	if len(models) == 0 {
		want = migrate.NewSchemaModel(schema)
	} else {
		p := parser.New(parserDialect, schema)
		want, pErr = p.Parse(models...)
		if pErr != nil {
			return nil, dialect, fmt.Errorf("parser: %w", pErr)
		}
	}
	var dbModel *migrate.SchemaModel
	if opts.DBDriver != nil {
		dbModel, pErr = opts.DBDriver.ReadSchema(ctx, schema)
		if pErr != nil {
			return nil, dialect, fmt.Errorf("read schema: %w", pErr)
		}
	} else {
		dbModel = migrate.NewSchemaModel(schema)
	}
	nextVersion, err := store.NextVersion(opts.OutputDir)
	if err != nil {
		return nil, dialect, err
	}
	specs := []migrate.RenameSpec{}
	if opts.Migrator != nil {
		specs = opts.Migrator.RenameSpecs()
	}
	engine := diff.New(specs...)
	engine.NoSQL = (dialect == "mongodb")
	plan := engine.Compute(want, dbModel, nextVersion)
	return plan, dialect, nil
}

func pipelineStatus(r *Result) string {
	if len(r.Errors) > 0 {
		return "error"
	}
	if len(r.Warnings) > 0 {
		return "warning"
	}
	return "ok"
}
