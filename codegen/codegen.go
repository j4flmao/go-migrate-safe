// Package codegen renders a DiffPlan into .up.sql and .down.sql files.
//
// It is intentionally deterministic: the same plan + dialect always produces
// the same bytes. This is what allows the checksum mechanism in the file
// header to detect post-apply tampering.
package codegen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// Generator turns a DiffPlan into SQL files for one dialect.
type Generator struct {
	Dialect string // "postgres" | "mysql" | "sqlite"
	OutDir  string
	Now     func() time.Time // overridable for deterministic tests
}

// New constructs a Generator. If outDir is empty it defaults to "./migrations".
func New(dialect, outDir string) *Generator {
	if outDir == "" {
		outDir = "./migrations"
	}
	return &Generator{Dialect: dialect, OutDir: outDir, Now: time.Now}
}

// Result describes what was written.
type Result struct {
	UpFile      string
	DownFile    string
	UpContent   string
	DownContent string
	Checksum    string
}

// Generate writes both files to disk and returns their paths plus contents.
// It also fills plan.GeneratedFiles and the SQL field of every operation.
func (g *Generator) Generate(plan *migrate.DiffPlan) (*Result, error) {
	if plan == nil || plan.IsEmpty {
		return nil, fmt.Errorf("codegen: refusing to generate for empty plan")
	}
	for i := range plan.Operations {
		sql, err := g.renderOp(&plan.Operations[i])
		if err != nil {
			return nil, fmt.Errorf("codegen: render op %d (%s): %w", i, plan.Operations[i].Kind, err)
		}
		plan.Operations[i].SQL = sql
	}

	ext := ".sql"
	if g.Dialect == "mongodb" {
		ext = ".jsonc"
	}

	var upContent, downContent string
	var checksum string

	upBody := g.renderBody(plan)
	checksum = sha256Hex(upBody)
	upHeader := g.renderHeader(plan, "UP", checksum)
	if g.Dialect == "mongodb" {
		// JSONC: body is pure JSON (no inline comments), header uses "//"
		upBody = g.renderBodyJSON(plan)
	}
	upContent = ensureTrailingNewline(stripTrailingWS(upHeader + upBody))

	if g.Dialect == "mongodb" {
		downBody := g.renderDownStubJSON(plan)
		downHeader := g.renderHeader(plan, "DOWN", "")
		downContent = ensureTrailingNewline(stripTrailingWS(downHeader + downBody))
	} else {
		downStub := g.renderDownStub()
		downHeader := g.renderHeader(plan, "DOWN", "")
		downContent = ensureTrailingNewline(stripTrailingWS(downHeader + downStub))
	}

	if err := os.MkdirAll(g.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("codegen: mkdir %q: %w", g.OutDir, err)
	}
	upPath := filepath.Join(g.OutDir, fmt.Sprintf("%04d_%s.up%s", plan.Version, plan.Name, ext))
	downPath := filepath.Join(g.OutDir, fmt.Sprintf("%04d_%s.down%s", plan.Version, plan.Name, ext))

	if err := os.WriteFile(upPath, []byte(upContent), 0o644); err != nil {
		return nil, fmt.Errorf("codegen: write %q: %w", upPath, err)
	}
	if err := os.WriteFile(downPath, []byte(downContent), 0o644); err != nil {
		return nil, fmt.Errorf("codegen: write %q: %w", downPath, err)
	}
	plan.GeneratedFiles = []string{upPath, downPath}
	return &Result{
		UpFile: upPath, DownFile: downPath,
		UpContent: upContent, DownContent: downContent,
		Checksum: checksum,
	}, nil
}

// RenderOnly produces the up/down content without writing to disk. Useful for
// golden-file tests.
func (g *Generator) RenderOnly(plan *migrate.DiffPlan) (*Result, error) {
	for i := range plan.Operations {
		sql, err := g.renderOp(&plan.Operations[i])
		if err != nil {
			return nil, err
		}
		plan.Operations[i].SQL = sql
	}
	upBody := g.renderBody(plan)
	checksum := sha256Hex(upBody)
	if g.Dialect == "mongodb" {
		upBody = g.renderBodyJSON(plan)
		downBody := g.renderDownStubJSON(plan)
		upContent := ensureTrailingNewline(stripTrailingWS(g.renderHeader(plan, "UP", checksum) + upBody))
		downContent := ensureTrailingNewline(stripTrailingWS(g.renderHeader(plan, "DOWN", "") + downBody))
		return &Result{UpContent: upContent, DownContent: downContent, Checksum: checksum}, nil
	}
	upContent := ensureTrailingNewline(stripTrailingWS(g.renderHeader(plan, "UP", checksum) + upBody))
	downContent := ensureTrailingNewline(stripTrailingWS(g.renderHeader(plan, "DOWN", "") + g.renderDownStub()))
	return &Result{UpContent: upContent, DownContent: downContent, Checksum: checksum}, nil
}

func (g *Generator) commentPrefix() string {
	if g.Dialect == "mongodb" {
		return "//"
	}
	return "--"
}

func (g *Generator) renderHeader(plan *migrate.DiffPlan, dir, checksum string) string {
	c := g.commentPrefix()
	var b strings.Builder
	fmt.Fprintf(&b, "%s go-migrate-safe v%s\n", c, migrate.Version)
	fmt.Fprintf(&b, "%s Version: %d\n", c, plan.Version)
	fmt.Fprintf(&b, "%s Name: %s\n", c, plan.Name)
	fmt.Fprintf(&b, "%s Direction: %s\n", c, dir)
	fmt.Fprintf(&b, "%s Generated: %s\n", c, g.Now().UTC().Format(time.RFC3339))
	if checksum != "" {
		fmt.Fprintf(&b, "%s Checksum: sha256:%s\n", c, checksum)
	}
	fmt.Fprintf(&b, "%s\n", c)
	fmt.Fprintf(&b, "%s Operations (%d):\n", c, len(plan.Operations))
	for i, op := range plan.Operations {
		col := ""
		if op.Column != "" {
			col = "." + op.Column
		}
		fmt.Fprintf(&b, "%s   [%d] %s: %s%s\n", c, i+1, op.Kind, op.Table, col)
	}
	fmt.Fprintf(&b, "%s\n", c)
	fmt.Fprintf(&b, "%s DO NOT EDIT this file after it has been applied to any environment.\n", c)
	fmt.Fprintf(&b, "%s To change the schema, create a new migration.\n\n", c)
	return b.String()
}

func (g *Generator) renderBody(plan *migrate.DiffPlan) string {
	c := g.commentPrefix()
	var b strings.Builder

	var statements []string
	switch g.Dialect {
	case "postgres":
		statements = RenderPostgresBatch(plan.Operations)
	case "mysql":
		statements = RenderMySQLBatch(plan.Operations)
	default:
		for _, op := range plan.Operations {
			if op.SQL != "" {
				statements = append(statements, op.SQL)
			}
		}
	}

	// For comments and reasoning, we still want to show what each op was.
	// However, if we batched them, the SQL statements don't map 1:1 to operations.
	// So we'll list all operations as comments at the top of the body, then the statements.
	fmt.Fprintf(&b, "%s Logic: Grouped by table for performance\n", c)
	for i, op := range plan.Operations {
		col := ""
		if op.Column != "" {
			col = "." + op.Column
		}
		fmt.Fprintf(&b, "%s [%d/%d] %s: %s%s (Reason: %s)\n", c, i+1, len(plan.Operations), op.Kind, op.Table, col, op.Reason)
	}
	b.WriteByte('\n')

	for _, stmt := range statements {
		b.WriteString(stmt)
		if !strings.HasSuffix(stmt, "\n") {
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func (g *Generator) renderBodyJSON(plan *migrate.DiffPlan) string {
	var items []string
	for _, op := range plan.Operations {
		if op.SQL == "" {
			continue
		}
		items = append(items, strings.TrimRight(op.SQL, "\n"))
	}
	if len(items) == 0 {
		return ""
	}
	return "[\n  " + strings.Join(items, ",\n  ") + "\n]\n"
}

func (g *Generator) renderDownStubJSON(plan *migrate.DiffPlan) string {
	var items []string
	for i := len(plan.Operations) - 1; i >= 0; i-- {
		op := plan.Operations[i]
		switch op.Kind {
		case migrate.OpAddTable:
			items = append(items, fmt.Sprintf(`{"drop": %q}`, op.Table))
		case migrate.OpAddIndex:
			if op.Index != "" {
				items = append(items, fmt.Sprintf(`{"dropIndexes": %q, "index": %q}`, op.Table, op.Index))
			}
		}
	}
	if len(items) == 0 {
		return ""
	}
	return "[\n  " + strings.Join(items, ",\n  ") + "\n]\n"
}

func (g *Generator) renderDownStub() string {
	return "-- Rollback SQL to be populated by Rollback Agent.\n"
}

func (g *Generator) renderOp(op *migrate.Operation) (string, error) {
	switch g.Dialect {
	case "postgres", "":
		return RenderPostgres(op)
	case "mysql":
		return RenderMySQL(op)
	case "sqlite":
		return RenderSQLite(op)
	case "mssql":
		return RenderMSSQL(op)
	case "mongodb":
		return RenderMongoDB(op)
	default:
		return "", fmt.Errorf("%w: %q", migrate.ErrUnsupportedDriver, g.Dialect)
	}
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func stripTrailingWS(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

func ensureTrailingNewline(s string) string {
	s = strings.TrimRight(s, "\n")
	return s + "\n"
}
