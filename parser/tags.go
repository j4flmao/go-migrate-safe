package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// Tag is the parsed form of a single `db:"..."` struct tag.
//
// Supported syntax:
//
//	db:"column_name,pk,autoincrement,not null,unique,index,default:NOW(),size:50,type:JSONB,fk:other_table(id),ignore"
//
// Tokens are comma-separated. Tokens with values use "key:value" form.
type Tag struct {
	Raw           string
	ColumnName    string
	Ignore        bool
	IsPK          bool
	AutoIncrement bool
	NotNull       bool // explicit not-null tag (otherwise inferred from Go type)
	Nullable      bool // explicit "null" tag (overrides Go type)
	Unique        bool
	Index         bool
	IndexName     string // when "index:my_idx"
	UniqueName    string // when "unique:my_uniq"
	Default       *string
	Size          *int
	Precision     *int
	Scale         *int
	TypeOverride  string // raw SQL type override
	TableOverride string // when "table:user_profile"
	TableOldName  string // when "table_old_name:user_profile_old"

	// FK references: "fk:other_table(id)" or "fk:other_table"
	FKRefTable  string
	FKRefColumn string

	OldName string // when "old_name:xyz"
	Check   string // when "check:price > 0"
}

// ParseTag parses a `db:"..."` tag string. Empty tag returns Tag{}.
func ParseTag(raw string) (Tag, error) {
	t := Tag{Raw: raw}
	if raw == "" {
		return t, nil
	}
	tokens := splitTopLevel(raw, ',')
	for i, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if i == 0 && !strings.Contains(tok, ":") && !isReservedKeyword(tok) {
			t.ColumnName = tok
			continue
		}
		if err := t.applyToken(tok); err != nil {
			return t, fmt.Errorf("tag %q: %w", raw, err)
		}
	}
	return t, nil
}

func (t *Tag) applyToken(tok string) error {
	low := strings.ToLower(tok)
	switch {
	case low == "ignore", low == "-":
		t.Ignore = true
	case low == "pk", low == "primary key":
		t.IsPK = true
	case low == "autoincrement", low == "auto_increment", low == "serial":
		t.AutoIncrement = true
	case low == "not null", low == "notnull":
		t.NotNull = true
	case low == "null", low == "nullable":
		t.Nullable = true
	case low == "unique":
		t.Unique = true
	case low == "index":
		t.Index = true
	case strings.HasPrefix(low, "default:"):
		v := tok[len("default:"):]
		t.Default = &v
	case strings.HasPrefix(low, "size:"):
		n, err := strconv.Atoi(tok[len("size:"):])
		if err != nil {
			return fmt.Errorf("invalid size: %v", err)
		}
		t.Size = &n
	case strings.HasPrefix(low, "precision:"):
		n, err := strconv.Atoi(tok[len("precision:"):])
		if err != nil {
			return fmt.Errorf("invalid precision: %v", err)
		}
		t.Precision = &n
	case strings.HasPrefix(low, "scale:"):
		n, err := strconv.Atoi(tok[len("scale:"):])
		if err != nil {
			return fmt.Errorf("invalid scale: %v", err)
		}
		t.Scale = &n
	case strings.HasPrefix(low, "type:"):
		t.TypeOverride = tok[len("type:"):]
	case strings.HasPrefix(low, "table:"):
		t.TableOverride = tok[len("table:"):]
	case strings.HasPrefix(low, "table_old_name:"):
		t.TableOldName = tok[len("table_old_name:"):]
	case strings.HasPrefix(low, "old_name:"):
		t.OldName = tok[len("old_name:"):]
	case strings.HasPrefix(low, "check:"):
		t.Check = tok[len("check:"):]
	case strings.HasPrefix(low, "index:"):
		t.Index = true
		t.IndexName = tok[len("index:"):]
	case strings.HasPrefix(low, "unique:"):
		t.Unique = true
		t.UniqueName = tok[len("unique:"):]
	case strings.HasPrefix(low, "fk:"):
		ref := tok[len("fk:"):]
		if i := strings.IndexByte(ref, '('); i > 0 && strings.HasSuffix(ref, ")") {
			t.FKRefTable = ref[:i]
			t.FKRefColumn = ref[i+1 : len(ref)-1]
		} else {
			t.FKRefTable = ref
			t.FKRefColumn = "id"
		}
	default:
		return fmt.Errorf("unknown tag token: %q", tok)
	}
	return nil
}

// splitTopLevel splits s on sep, ignoring sep inside parentheses.
func splitTopLevel(s string, sep byte) []string {
	out := []string{}
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case sep:
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

func isReservedKeyword(tok string) bool {
	low := strings.ToLower(strings.TrimSpace(tok))
	switch low {
	case "ignore", "-", "pk", "primary key", "autoincrement", "auto_increment",
		"serial", "not null", "notnull", "null", "nullable", "unique", "index":
		return true
	}
	for _, prefix := range []string{
		"default:", "size:", "precision:", "scale:", "type:",
		"table:", "table_old_name:", "old_name:", "check:", "index:", "unique:", "fk:",
	} {
		if strings.HasPrefix(low, prefix) {
			return true
		}
	}
	return false
}
