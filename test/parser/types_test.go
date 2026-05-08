package parser_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/parser"
)

func TestNormalizeType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"character varying", "TEXT"},
		{"varchar", "TEXT"},
		{"int4", "INTEGER"},
		{"int", "INTEGER"},
		{"int8", "BIGINT"},
		{"bool", "BOOLEAN"},
		{"tinyint(1)", "BOOLEAN"},
		{"timestamp with time zone", "TIMESTAMPTZ"},
		{"timestamp without time zone", "TIMESTAMP"},
		{"double precision", "DOUBLE"},
		{"VARCHAR(50)", "VARCHAR(50)"},
		{"DECIMAL(12,4)", "DECIMAL(12,4)"},
		{"longtext", "TEXT"},
	}
	for _, c := range cases {
		if got := parser.NormalizeType(c.in); got != c.want {
			t.Errorf("NormalizeType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
