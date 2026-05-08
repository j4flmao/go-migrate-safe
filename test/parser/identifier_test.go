package parser_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/parser"
)

func TestSnakeCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"User", "user"},
		{"UserID", "user_id"},
		{"OrderItem", "order_item"},
		{"UserProfile", "user_profile"},
		{"HTTPStatus", "http_status"},
		{"CreatedAt", "created_at"},
		{"IsActive", "is_active"},
		{"ID", "id"},
		{"", ""},
	}
	for _, c := range cases {
		if got := parser.SnakeCase(c.in); got != c.want {
			t.Errorf("SnakeCase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPluralizeTable(t *testing.T) {
	cases := []struct{ in, want string }{
		{"User", "users"},
		{"OrderItem", "order_items"},
		{"UserProfile", "user_profiles"},
		{"Box", "boxes"},
		{"Class", "classes"},
		{"Country", "countries"},
		{"Day", "days"},
	}
	for _, c := range cases {
		if got := parser.PluralizeTable(c.in); got != c.want {
			t.Errorf("PluralizeTable(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidateIdentifier(t *testing.T) {
	good := []string{"users", "order_items", "u1", "abc_123_def"}
	bad := []string{"", "Users", "1abc", "user-name", "user.name", "user;drop", "_x"}
	for _, s := range good {
		if err := parser.ValidateIdentifier(s); err != nil {
			t.Errorf("ValidateIdentifier(%q) unexpected error: %v", s, err)
		}
	}
	for _, s := range bad {
		if err := parser.ValidateIdentifier(s); err == nil {
			t.Errorf("ValidateIdentifier(%q) expected error, got nil", s)
		}
	}
}
