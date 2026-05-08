package postgres_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/postgres"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestDriverInterfaceCompileCheck(t *testing.T) {
	var _ migrate.Driver = (*postgres.Driver)(nil)
}

func TestNew(t *testing.T) {
	d := postgres.New(nil)
	if d == nil {
		t.Fatal("New returned nil")
	}
}

func TestDriverName(t *testing.T) {
	d := postgres.New(nil)
	if got := d.DriverName(); got != "postgres" {
		t.Errorf("DriverName = %q, want postgres", got)
	}
}
