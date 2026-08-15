// Package diff implements the schema diff engine.
//
// Given two SchemaModel values (struct model = desired state, db model =
// current state) it produces an ordered DiffPlan describing the changes
// needed to bring the DB into sync with the structs.
package diff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/j4flmao/go-migrate-safe/migrate"
)

// Engine computes diffs between two SchemaModel values.
type Engine struct {
	// RenameSpecs are explicit user-declared renames that suppress drop+add
	// pairs and emit OpRenameColumn instead.
	RenameSpecs []migrate.RenameSpec
	// NoSQL skips column-level DDL (suitable for MongoDB etc.).
	NoSQL bool
}

// New constructs a diff engine.
func New(specs ...migrate.RenameSpec) *Engine {
	return &Engine{RenameSpecs: specs}
}

// Compute produces a DiffPlan that describes how to transform db into want.
func (e *Engine) Compute(want, db *migrate.SchemaModel, nextVersion int64) *migrate.DiffPlan {
	plan := &migrate.DiffPlan{Version: nextVersion}
	if want == nil {
		want = migrate.NewSchemaModel("")
	}
	if db == nil {
		db = migrate.NewSchemaModel("")
	}

	wantTables := sortedKeys(want.Tables)
	dbTables := sortedKeys(db.Tables)

	// Apply explicit renames from Engine specs as a pre-pass.
	var renameOps []migrate.Operation
	var renameTableOps []migrate.Operation
	for _, r := range e.RenameSpecs {
		t, ok := db.Tables[r.Table]
		if !ok {
			continue
		}
		c, ok := t.Columns[r.OldName]
		if !ok {
			continue
		}
		if _, exists := t.Columns[r.NewName]; exists {
			continue
		}
		nc := *c
		nc.Name = r.NewName
		t.Columns[r.NewName] = &nc
		delete(t.Columns, r.OldName)
		// Update ColumnOrder so subsequent diff iteration does not emit a
		// spurious DropColumn for the renamed-away name.
		for i, name := range t.ColumnOrder {
			if name == r.OldName {
				t.ColumnOrder[i] = r.NewName
				break
			}
		}
		renameOps = append(renameOps, migrate.Operation{
			Kind:   migrate.OpRenameColumn,
			Table:  r.Table,
			Column: r.NewName,
			Reason: fmt.Sprintf("Rename column %q.%q → %q (explicit WithRenameColumn)", r.Table, r.OldName, r.NewName),
			Before: c,
			After:  &nc,
		})
	}

	// Apply struct tag table renames
	for _, name := range wantTables {
		wt := want.Tables[name]
		if wt.OldName != "" {
			if dt, exists := db.Tables[wt.OldName]; exists {
				if _, conflict := db.Tables[name]; !conflict {
					renameTableOps = append(renameTableOps, migrate.Operation{
						Kind:     migrate.OpRenameTable,
						Table:    wt.OldName,
						NewTable: wt,
						Reason:   fmt.Sprintf("Rename table %q → %q (explicit struct tag)", wt.OldName, name),
					})
					dt.Name = name
					db.Tables[name] = dt
					delete(db.Tables, wt.OldName)
				}
			}
		}
	}

	// Apply struct tag column renames
	for _, name := range wantTables {
		wt := want.Tables[name]
		dt, ok := db.Tables[name]
		if !ok {
			continue
		}
		for _, cname := range orderedColumnNames(wt) {
			wc := wt.Columns[cname]
			if wc.OldName != "" {
				if dc, exists := dt.Columns[wc.OldName]; exists {
					if _, conflict := dt.Columns[cname]; !conflict {
						nc := *dc
						nc.Name = cname
						dt.Columns[cname] = &nc
						delete(dt.Columns, wc.OldName)
						for i, oldCol := range dt.ColumnOrder {
							if oldCol == wc.OldName {
								dt.ColumnOrder[i] = cname
								break
							}
						}
						renameOps = append(renameOps, migrate.Operation{
							Kind:   migrate.OpRenameColumn,
							Table:  name,
							Column: cname,
							Reason: fmt.Sprintf("Rename column %q.%q → %q (explicit struct tag)", name, wc.OldName, cname),
							Before: dc,
							After:  &nc,
						})
					}
				}
			}
		}
	}

	// Re-evaluate dbTables after table renames
	dbTables = sortedKeys(db.Tables)

	// Buckets so we can emit them in the safe order required by §3.2.
	var (
		dropFK         []migrate.Operation
		dropIdx        []migrate.Operation
		dropUniq       []migrate.Operation
		dropCol        []migrate.Operation
		dropTbl        []migrate.Operation
		addTbl         []migrate.Operation
		addColSafe     []migrate.Operation
		addColUnsafe   []migrate.Operation
		addIdx         []migrate.Operation
		addFK          []migrate.Operation
		addUniq        []migrate.Operation
		alterCol       []migrate.Operation
		dropConstraint []migrate.Operation
		addConstraint  []migrate.Operation
	)

	// --- Tables only in want → ADD TABLE ---
	for _, name := range wantTables {
		if _, ok := db.Tables[name]; !ok {
			t := want.Tables[name]
			addTbl = append(addTbl, migrate.Operation{
				Kind:     migrate.OpAddTable,
				Table:    name,
				NewTable: t,
				Reason:   fmt.Sprintf("Added %q table (%d columns) — matches new struct", name, len(t.Columns)),
			})
			if e.NoSQL {
				// NoSQL drivers need separate index/constraint operations
				// because collection creation doesn't include them.
				for _, iname := range sortedIdxKeys(t.Indexes) {
					addIdx = append(addIdx, migrate.Operation{
						Kind:     migrate.OpAddIndex,
						Table:    name,
						Index:    iname,
						IndexDef: t.Indexes[iname],
						Reason:   fmt.Sprintf("Added index %q on %s(%s)", iname, name, strings.Join(t.Indexes[iname].Columns, ",")),
					})
				}
				for _, cname := range sortedCstrKeys(t.Constraints) {
					wc := t.Constraints[cname]
					op := migrate.Operation{
						Kind:          migrate.OpAddConstraint,
						Table:         name,
						ConstraintDef: wc,
						Reason:        fmt.Sprintf("Added constraint %q (%s)", cname, wc.Kind),
					}
					switch wc.Kind {
					case migrate.ConstraintForeignKey:
						addFK = append(addFK, op)
					case migrate.ConstraintUnique:
						addUniq = append(addUniq, op)
					default:
						addConstraint = append(addConstraint, op)
					}
				}
			}
		}
	}

	// --- Tables only in db → DROP TABLE ---
	for _, name := range dbTables {
		if _, ok := want.Tables[name]; !ok {
			dropTbl = append(dropTbl, migrate.Operation{
				Kind:     migrate.OpDropTable,
				Table:    name,
				IsUnsafe: true,
				Reason:   fmt.Sprintf("Removed %q table — no corresponding struct model found", name),
			})
		}
	}

	// --- Tables in both → column / index / constraint diff ---
	for _, name := range wantTables {
		wt, ok := want.Tables[name]
		if !ok {
			continue
		}
		dt, ok := db.Tables[name]
		if !ok {
			continue
		}
		// Columns
		for _, cname := range orderedColumnNames(wt) {
			wc := wt.Columns[cname]
			dc, exists := dt.Columns[cname]
			if !exists {
				op := migrate.Operation{
					Kind:   migrate.OpAddColumn,
					Table:  name,
					Column: cname,
					After:  wc,
					Reason: fmt.Sprintf("Added column %q.%q %s — new field in struct", name, cname, wc.SQLType),
				}
				if !wc.Nullable && wc.Default == nil {
					op.IsUnsafe = true // destructive if table has data
					addColUnsafe = append(addColUnsafe, op)
				} else {
					addColSafe = append(addColSafe, op)
				}
				continue
			}
			if changed, reason := columnDiffers(dc, wc); changed {
				op := migrate.Operation{
					Kind:   migrate.OpAlterColumn,
					Table:  name,
					Column: cname,
					Before: dc,
					After:  wc,
					Reason: fmt.Sprintf("Changed %q.%q: %s", name, cname, reason),
				}
				if isLossyTypeChange(dc.SQLType, wc.SQLType) {
					op.IsUnsafe = true
				}
				if dc.Nullable && !wc.Nullable {
					// nullable → not null may fail if NULLs exist
					op.IsUnsafe = true
				}
				alterCol = append(alterCol, op)
			}
		}
		for _, cname := range orderedColumnNames(dt) {
			if _, ok := wt.Columns[cname]; !ok {
				dropCol = append(dropCol, migrate.Operation{
					Kind:     migrate.OpDropColumn,
					Table:    name,
					Column:   cname,
					Before:   dt.Columns[cname],
					IsUnsafe: true,
					Reason:   fmt.Sprintf("Removed column %q.%q — field removed from struct ⚠ data will be lost", name, cname),
				})
			}
		}

		// Indexes
		for _, iname := range sortedIdxKeys(wt.Indexes) {
			wi := wt.Indexes[iname]
			if _, ok := dt.Indexes[iname]; !ok {
				addIdx = append(addIdx, migrate.Operation{
					Kind:     migrate.OpAddIndex,
					Table:    name,
					Index:    iname,
					IndexDef: wi,
					Reason:   fmt.Sprintf("Added index %q on %s(%s)", iname, name, strings.Join(wi.Columns, ",")),
				})
			}
		}
		for _, iname := range sortedIdxKeys(dt.Indexes) {
			di := dt.Indexes[iname]
			if _, ok := wt.Indexes[iname]; !ok {
				dropIdx = append(dropIdx, migrate.Operation{
					Kind:     migrate.OpDropIndex,
					Table:    name,
					Index:    iname,
					IndexDef: di,
					Reason:   fmt.Sprintf("Removed index %q on %s", iname, name),
				})
			}
		}

		// Constraints (FK / unique)
		for _, cname := range sortedCstrKeys(wt.Constraints) {
			wc := wt.Constraints[cname]
			if _, ok := dt.Constraints[cname]; !ok {
				op := migrate.Operation{
					Kind:          migrate.OpAddConstraint,
					Table:         name,
					ConstraintDef: wc,
					Reason:        fmt.Sprintf("Added constraint %q (%s)", cname, wc.Kind),
				}
				switch wc.Kind {
				case migrate.ConstraintForeignKey:
					addFK = append(addFK, op)
				case migrate.ConstraintUnique:
					addUniq = append(addUniq, op)
				default:
					addConstraint = append(addConstraint, op)
				}
			}
		}
		for _, cname := range sortedCstrKeys(dt.Constraints) {
			dc := dt.Constraints[cname]
			if _, ok := wt.Constraints[cname]; !ok {
				op := migrate.Operation{
					Kind:          migrate.OpDropConstraint,
					Table:         name,
					ConstraintDef: dc,
					Reason:        fmt.Sprintf("Removed constraint %q (%s)", cname, dc.Kind),
				}
				switch dc.Kind {
				case migrate.ConstraintForeignKey:
					dropFK = append(dropFK, op)
				case migrate.ConstraintUnique:
					op.IsUnsafe = true // dropping unique constraint allows duplicates, making rollback unsafe
					dropUniq = append(dropUniq, op)
				default:
					dropConstraint = append(dropConstraint, op)
				}
			}
		}
	}

	// Assemble in safe order (data-model.md §2.2).
	plan.Operations = append(plan.Operations, dropFK...)
	plan.Operations = append(plan.Operations, dropIdx...)
	plan.Operations = append(plan.Operations, dropUniq...)
	plan.Operations = append(plan.Operations, dropConstraint...)
	plan.Operations = append(plan.Operations, dropCol...)
	plan.Operations = append(plan.Operations, dropTbl...)
	plan.Operations = append(plan.Operations, addTbl...)
	plan.Operations = append(plan.Operations, addColSafe...)
	plan.Operations = append(plan.Operations, addColUnsafe...)
	plan.Operations = append(plan.Operations, addIdx...)
	plan.Operations = append(plan.Operations, addUniq...)
	plan.Operations = append(plan.Operations, addFK...)
	plan.Operations = append(plan.Operations, addConstraint...)
	plan.Operations = append(plan.Operations, alterCol...)
	plan.Operations = append(renameTableOps, plan.Operations...)
	plan.Operations = append(renameOps, plan.Operations...)

	// Destructive flag.
	for _, op := range plan.Operations {
		if op.IsUnsafe {
			plan.HasDestructiveOps = true
			break
		}
	}

	plan.Name = deriveName(plan.Operations)
	plan.IsEmpty = len(plan.Operations) == 0
	plan.RenameHints = computeRenameHints(dropCol, addColSafe, addColUnsafe)
	return plan
}

func columnDiffers(a, b *migrate.ColumnModel) (bool, string) {
	var reasons []string
	if a.SQLType != b.SQLType {
		reasons = append(reasons, fmt.Sprintf("%s → %s", a.SQLType, b.SQLType))
	}
	if a.Nullable != b.Nullable {
		if a.Nullable {
			reasons = append(reasons, "nullable → NOT NULL")
		} else {
			reasons = append(reasons, "NOT NULL → nullable")
		}
	}
	if !ptrEq(a.Default, b.Default) {
		reasons = append(reasons, "default changed")
	}
	if !intPtrEq(a.Size, b.Size) {
		reasons = append(reasons, "size changed")
	}
	if len(reasons) == 0 {
		return false, ""
	}
	return true, strings.Join(reasons, "; ")
}

func ptrEq(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
func intPtrEq(a, b *int) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

// isLossyTypeChange returns true when changing column type from a→b risks data loss.
func isLossyTypeChange(a, b string) bool {
	a = strings.ToUpper(a)
	b = strings.ToUpper(b)
	if a == b {
		return false
	}

	// Numeric widening (safe)
	rank := map[string]int{
		"SMALLINT": 1, "INT2": 1,
		"INTEGER": 2, "INT": 2, "INT4": 2,
		"BIGINT": 3, "INT8": 3,
		"FLOAT4": 4, "REAL": 4,
		"FLOAT8": 5, "DOUBLE PRECISION": 5, "DOUBLE": 5,
	}

	if ar, ok := rank[a]; ok {
		if br, ok := rank[b]; ok {
			return br < ar // narrowing is lossy
		}
	}

	// String widening (safe)
	if (strings.HasPrefix(a, "VARCHAR") || strings.HasPrefix(a, "CHAR")) &&
		(strings.HasPrefix(b, "VARCHAR") || strings.HasPrefix(b, "CHAR") || b == "TEXT" || b == "LONGTEXT") {
		if b == "TEXT" || b == "LONGTEXT" {
			return false // widening to TEXT is always safe
		}
		return varcharSize(b) < varcharSize(a)
	}

	// TEXT to VARCHAR is lossy (truncation risk)
	if (a == "TEXT" || a == "LONGTEXT") && (strings.HasPrefix(b, "VARCHAR") || strings.HasPrefix(b, "CHAR")) {
		return true
	}

	// Date/Time safe conversions
	if a == "DATE" && (b == "TIMESTAMP" || b == "TIMESTAMPTZ" || b == "DATETIME") {
		return false
	}

	return true // unknown -> assume lossy for safety
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

func deriveName(ops []migrate.Operation) string {
	if len(ops) == 0 {
		return "noop"
	}
	first := opName(ops[0])
	if len(ops) == 1 {
		return first
	}
	second := opName(ops[1])
	combined := first + "_and_" + second
	if len(combined) > 60 {
		return first + "_and_more"
	}
	return combined
}

func opName(op migrate.Operation) string {
	switch op.Kind {
	case migrate.OpAddTable:
		return "add_" + op.Table + "_table"
	case migrate.OpDropTable:
		return "drop_" + op.Table + "_table"
	case migrate.OpAddColumn:
		return "add_" + op.Table + "_" + op.Column
	case migrate.OpDropColumn:
		return "drop_" + op.Table + "_" + op.Column
	case migrate.OpAlterColumn:
		return "alter_" + op.Table + "_" + op.Column
	case migrate.OpRenameColumn:
		return "rename_" + op.Table + "_" + op.Column
	case migrate.OpRenameTable:
		return "rename_" + op.Table + "_to_" + op.NewTable.Name
	case migrate.OpAddIndex:
		return "add_" + op.Index
	case migrate.OpDropIndex:
		return "drop_" + op.Index
	case migrate.OpAddConstraint:
		if op.ConstraintDef != nil {
			return "add_" + string(op.ConstraintDef.Kind) + "_" + op.Table
		}
		return "add_constraint_" + op.Table
	case migrate.OpDropConstraint:
		return "drop_constraint_" + op.Table
	}
	return "change_" + op.Table
}

func computeRenameHints(drops, addsSafe, addsUnsafe []migrate.Operation) []migrate.RenameHint {
	adds := append([]migrate.Operation{}, addsSafe...)
	adds = append(adds, addsUnsafe...)
	byTableDrops := map[string][]migrate.Operation{}
	byTableAdds := map[string][]migrate.Operation{}
	for _, op := range drops {
		byTableDrops[op.Table] = append(byTableDrops[op.Table], op)
	}
	for _, op := range adds {
		byTableAdds[op.Table] = append(byTableAdds[op.Table], op)
	}
	var hints []migrate.RenameHint
	for tbl, ds := range byTableDrops {
		as := byTableAdds[tbl]
		if len(ds) != 1 || len(as) != 1 {
			continue
		}
		d := ds[0]
		a := as[0]
		if d.Before == nil || a.After == nil {
			continue
		}
		if d.Before.SQLType != a.After.SQLType {
			continue
		}
		conf := "medium"
		if editDistance(d.Column, a.Column) < 3 {
			conf = "high"
		}
		hints = append(hints, migrate.RenameHint{
			Table:         tbl,
			DroppedColumn: d.Column,
			AddedColumn:   a.Column,
			Confidence:    conf,
			Reason:        fmt.Sprintf("One column dropped (%s) and one added (%s) of same type %s", d.Column, a.Column, d.Before.SQLType),
		})
	}
	return hints
}

func editDistance(a, b string) int {
	la, lb := len(a), len(b)
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// --- helpers ---

func sortedKeys(m map[string]*migrate.TableModel) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func sortedIdxKeys(m map[string]*migrate.IndexModel) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func sortedCstrKeys(m map[string]*migrate.ConstraintModel) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func orderedColumnNames(t *migrate.TableModel) []string {
	if len(t.ColumnOrder) == len(t.Columns) {
		return t.ColumnOrder
	}
	out := make([]string, 0, len(t.Columns))
	for k := range t.Columns {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
