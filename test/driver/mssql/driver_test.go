package mssql_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/mssql"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestDriverInterfaceCompileCheck(t *testing.T) {
	var _ migrate.Driver = (*mssql.Driver)(nil)
}

func TestNew(t *testing.T) {
	d := mssql.New(nil)
	if d == nil {
		t.Fatal("New returned nil")
	}
}

func TestDriverName(t *testing.T) {
	d := mssql.New(nil)
	if got := d.DriverName(); got != "mssql" {
		t.Errorf("DriverName = %q, want mssql", got)
	}
}
