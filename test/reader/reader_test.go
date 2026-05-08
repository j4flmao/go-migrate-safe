package reader_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/reader"
)

func TestNewPostgres(t *testing.T) {
	r := reader.NewPostgres(nil)
	if r == nil {
		t.Fatal("NewPostgres returned nil")
	}
}

func TestNewMySQL(t *testing.T) {
	r := reader.NewMySQL(nil)
	if r == nil {
		t.Fatal("NewMySQL returned nil")
	}
}

func TestNewSQLite(t *testing.T) {
	r := reader.NewSQLite(nil)
	if r == nil {
		t.Fatal("NewSQLite returned nil")
	}
}

func TestReaderInterfaceCompileCheck(t *testing.T) {
	var _ reader.Reader = reader.NewPostgres(nil)
	var _ reader.Reader = reader.NewMySQL(nil)
	var _ reader.Reader = reader.NewSQLite(nil)
}

func TestPostgresReader_ReadSchema_NilDB(t *testing.T) {
	r := reader.NewPostgres(nil)
	_, err := r.ReadSchema(context.Background(), "public")
	if err == nil {
		t.Fatal("expected error with nil db")
	}
}

func TestMySQLReader_ReadSchema_NilDB(t *testing.T) {
	r := reader.NewMySQL(nil)
	_, err := r.ReadSchema(context.Background(), "public")
	if err == nil {
		t.Fatal("expected error with nil db")
	}
}

func TestSQLiteReader_ReadSchema_NilDB(t *testing.T) {
	r := reader.NewSQLite(nil)
	_, err := r.ReadSchema(context.Background(), "public")
	if err == nil {
		t.Fatal("expected error with nil db")
	}
}
