// Package parser converts Go structs (with `db:"..."` tags) into a SchemaModel.
//
// It is internal to the library — users do not import this package directly.
package parser

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// identifierRe enforces the GR-SEC2 identifier allowlist for SQL identifiers.
var identifierRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ValidateIdentifier returns nil if s is a safe DDL identifier, else an error.
func ValidateIdentifier(s string) error {
	if !identifierRe.MatchString(s) {
		return fmt.Errorf("invalid identifier %q: must match [a-z][a-z0-9_]{0,62}", s)
	}
	return nil
}

// SnakeCase converts a Go identifier (PascalCase / camelCase) to snake_case.
// HTTPStatus → http_status, UserID → user_id, OrderItem → order_item.
func SnakeCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := runes[i-1]
				next := rune(0)
				if i+1 < len(runes) {
					next = runes[i+1]
				}
				// Insert underscore when:
				//   - previous rune was lowercase/digit (e.g. "Id" → "_id")
				//   - or this is upper and next is lowercase (acronym boundary, e.g. "HTTPStatus" → "HTTP_Status")
				if unicode.IsLower(prev) || unicode.IsDigit(prev) {
					b.WriteByte('_')
				} else if next != 0 && unicode.IsLower(next) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	out := b.String()
	// Collapse any leading underscore that may have been emitted by acronym handling.
	out = strings.TrimLeft(out, "_")
	return out
}

// PluralizeTable converts a singular Go type name into a plural snake_case
// table name. Simple English heuristic per NR-9.
func PluralizeTable(typeName string) string {
	s := SnakeCase(typeName)
	switch {
	case s == "":
		return s
	case strings.HasSuffix(s, "s") || strings.HasSuffix(s, "x") ||
		strings.HasSuffix(s, "z") || strings.HasSuffix(s, "ch") ||
		strings.HasSuffix(s, "sh"):
		return s + "es"
	case strings.HasSuffix(s, "y") && len(s) > 1 && !isVowel(s[len(s)-2]):
		return s[:len(s)-1] + "ies"
	default:
		return s + "s"
	}
}

func isVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}
