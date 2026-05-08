package sqlite_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/sqlite"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestSQLiteInterface(t *testing.T) {
	var _ migrate.Driver = (*sqlite.Driver)(nil)
}

func TestSQLiteNew(t *testing.T) {
	d := sqlite.New(nil)
	if d == nil {
		t.Fatal("New returned nil")
	}
}

func TestSQLiteDriverName(t *testing.T) {
	d := sqlite.New(nil)
	if got := d.DriverName(); got != "sqlite" {
		t.Errorf("DriverName = %q, want sqlite", got)
	}
}
