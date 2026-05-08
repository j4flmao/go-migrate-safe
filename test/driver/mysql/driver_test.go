package mysql_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/mysql"
	"github.com/j4flmao/go-migrate-safe/migrate"
)

func TestMySQLInterface(t *testing.T) {
	var _ migrate.Driver = (*mysql.Driver)(nil)
}

func TestMySQLNew(t *testing.T) {
	d := mysql.New(nil)
	if d == nil {
		t.Fatal("New returned nil")
	}
}

func TestMySQLDriverName(t *testing.T) {
	d := mysql.New(nil)
	if got := d.DriverName(); got != "mysql" {
		t.Errorf("DriverName = %q, want mysql", got)
	}
}
