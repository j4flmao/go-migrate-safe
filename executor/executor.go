// Package executor applies (or rolls back) migration files against a Driver.
//
// It enforces:
//   - advisory lock around the whole apply window
//   - transactional execution per migration
//   - history record (applied / failed / rolled_back) on every step
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/store"
)

// Executor applies and rolls back migrations.
type Executor struct {
	D     driver.Driver
	Now   func() time.Time
	DryRun bool
}

// New constructs an executor.
func New(d driver.Driver) *Executor {
	return &Executor{D: d, Now: time.Now}
}

// ApplyResult is returned from Apply.
type ApplyResult struct {
	Applied   []store.File
	Failed    *store.File
	StartedAt time.Time
	EndedAt   time.Time
	Err       error
}

// Apply runs each pending migration in order. Aborts on the first failure.
func (e *Executor) Apply(ctx context.Context, files []store.File) (*ApplyResult, error) {
	res := &ApplyResult{StartedAt: e.Now()}
	if err := e.D.EnsureHistoryTable(ctx); err != nil {
		return res, fmt.Errorf("apply: ensure history: %w", err)
	}
	if err := e.D.AcquireLock(ctx); err != nil {
		return res, fmt.Errorf("apply: %w", err)
	}
	defer func() { _ = e.D.ReleaseLock(ctx) }()

	for i := range files {
		f := files[i]
		if f.Direction != "up" {
			continue
		}
		start := e.Now()
		err := e.D.ExecTx(ctx, func(tx driver.Tx) error {
			for _, stmt := range splitByFormat(f.Body, f.Format) {
				if e.DryRun {
					continue
				}
				if err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		})
		dur := e.Now().Sub(start)
		rec := driver.MigrationRecord{
			Version:     f.Version,
			Name:        f.Name,
			Direction:   "up",
			Checksum:    f.Checksum,
			AppliedAt:   e.Now().UTC().Format(time.RFC3339),
			ExecutionMS: dur.Milliseconds(),
		}
		if err != nil {
			rec.Status = "failed"
			rec.ErrorMessage = err.Error()
			_ = e.D.RecordMigration(ctx, rec)
			res.Failed = &f
			res.Err = err
			res.EndedAt = e.Now()
			return res, fmt.Errorf("apply v%04d %q: %w", f.Version, f.Name, err)
		}
		rec.Status = "applied"
		if err := e.D.RecordMigration(ctx, rec); err != nil {
			res.Err = err
			return res, fmt.Errorf("apply v%04d: record: %w", f.Version, err)
		}
		res.Applied = append(res.Applied, f)
	}
	res.EndedAt = e.Now()
	return res, nil
}

// Rollback runs the .down.sql for each provided file (callers pass the down
// files in apply order; we reverse them here).
func (e *Executor) Rollback(ctx context.Context, downFiles []store.File) (*ApplyResult, error) {
	res := &ApplyResult{StartedAt: e.Now()}
	if err := e.D.EnsureHistoryTable(ctx); err != nil {
		return res, fmt.Errorf("rollback: ensure history: %w", err)
	}
	if err := e.D.AcquireLock(ctx); err != nil {
		return res, fmt.Errorf("rollback: %w", err)
	}
	defer func() { _ = e.D.ReleaseLock(ctx) }()

	for i := len(downFiles) - 1; i >= 0; i-- {
		f := downFiles[i]
		if f.Direction != "down" {
			continue
		}
		start := e.Now()
		err := e.D.ExecTx(ctx, func(tx driver.Tx) error {
			for _, stmt := range splitByFormat(f.Body, f.Format) {
				if e.DryRun {
					continue
				}
				if err := tx.Exec(ctx, stmt); err != nil {
					return err
				}
			}
			return nil
		})
		dur := e.Now().Sub(start)
		rec := driver.MigrationRecord{
			Version:     f.Version,
			Name:        f.Name,
			Direction:   "down",
			Checksum:    f.Checksum,
			AppliedAt:   e.Now().UTC().Format(time.RFC3339),
			ExecutionMS: dur.Milliseconds(),
			Status:      "rolled_back",
		}
		if err != nil {
			rec.Status = "failed"
			rec.ErrorMessage = err.Error()
			_ = e.D.RecordMigration(ctx, rec)
			res.Err = err
			res.Failed = &f
			res.EndedAt = e.Now()
			return res, fmt.Errorf("rollback v%04d %q: %w", f.Version, f.Name, err)
		}
		_ = e.D.RecordMigration(ctx, rec)
		res.Applied = append(res.Applied, f)
	}
	res.EndedAt = e.Now()
	return res, nil
}

// splitByFormat dispatches to the appropriate statement splitter.
func splitByFormat(body, format string) []string {
	switch format {
	case "json", "jsonc":
		return splitJSONCommands(body)
	case "js":
		return []string{body}
	default:
		return SplitStatements(body)
	}
}

// splitJSONCommands splits a JSONC migration body into individual commands.
// Supports two formats:
//   - JSON array:  [{"create":"x"},{"create":"y"}] — standard JSONC
//   - LDJSON/JSONL: {"create":"x"}\n{"create":"y"}   — legacy fallback
func splitJSONCommands(body string) []string {
	// Strip comment lines and empty lines
	var cleanLines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "#") {
			continue
		}
		cleanLines = append(cleanLines, line)
	}
	clean := strings.Join(cleanLines, "\n")
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return nil
	}

	// JSON array format — parse elements with decoder
	if strings.HasPrefix(clean, "[") {
		return splitJSONArray(clean)
	}

	// Legacy LDJSON — one command per line
	return strings.Split(clean, "\n")
}

// splitJSONArray extracts individual JSON values from a top-level array.
func splitJSONArray(body string) []string {
	dec := json.NewDecoder(strings.NewReader(body))
	t, err := dec.Token()
	if err != nil || t != json.Delim('[') {
		return []string{body}
	}
	var out []string
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			continue
		}
		out = append(out, string(raw))
	}
	dec.Token() // consume closing ]
	return out
}

// SplitStatements is a deliberately simple SQL splitter: it splits on
// statement-terminating semicolons that sit at the end of a non-comment line.
//
// It does not understand dollar-quoted strings or BEGIN..END blocks. Users
// who need those should use a single-statement migration.
func SplitStatements(body string) []string {
	var stmts []string
	var cur strings.Builder
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimRight(raw, "\r")
		t := strings.TrimSpace(line)
		if t == "" {
			if cur.Len() > 0 {
				cur.WriteByte('\n')
			}
			continue
		}
		if strings.HasPrefix(t, "--") {
			continue
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
		if strings.HasSuffix(t, ";") {
			stmt := strings.TrimSpace(cur.String())
			if stmt != "" {
				stmts = append(stmts, stmt)
			}
			cur.Reset()
		}
	}
	if leftover := strings.TrimSpace(cur.String()); leftover != "" {
		stmts = append(stmts, leftover)
	}
	return stmts
}
