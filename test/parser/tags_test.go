package parser_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/parser"
)

func TestParseTag_Basic(t *testing.T) {
	tag, err := parser.ParseTag("email,not null,unique,size:128")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.ColumnName != "email" {
		t.Errorf("ColumnName = %q", tag.ColumnName)
	}
	if !tag.NotNull {
		t.Errorf("NotNull = false, want true")
	}
	if !tag.Unique {
		t.Errorf("Unique = false, want true")
	}
	if tag.Size == nil || *tag.Size != 128 {
		t.Errorf("Size = %v, want 128", tag.Size)
	}
}

func TestParseTag_Default(t *testing.T) {
	tag, err := parser.ParseTag("created_at,not null,default:NOW()")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.Default == nil || *tag.Default != "NOW()" {
		t.Errorf("Default = %v, want NOW()", tag.Default)
	}
}

func TestParseTag_PKAutoincrement(t *testing.T) {
	tag, err := parser.ParseTag("id,pk,autoincrement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tag.IsPK {
		t.Errorf("IsPK = false")
	}
	if !tag.AutoIncrement {
		t.Errorf("AutoIncrement = false")
	}
}

func TestParseTag_Ignore(t *testing.T) {
	tag, err := parser.ParseTag("ignore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tag.Ignore {
		t.Errorf("Ignore = false")
	}
}

func TestParseTag_TypeOverride(t *testing.T) {
	tag, err := parser.ParseTag("metadata,type:JSONB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.TypeOverride != "JSONB" {
		t.Errorf("TypeOverride = %q", tag.TypeOverride)
	}
}

func TestParseTag_FK(t *testing.T) {
	tag, err := parser.ParseTag("user_id,not null,fk:users(id)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag.FKRefTable != "users" || tag.FKRefColumn != "id" {
		t.Errorf("FK ref = %q.%q", tag.FKRefTable, tag.FKRefColumn)
	}
}

func TestParseTag_Composite(t *testing.T) {
	tag, err := parser.ParseTag("user_id,index:idx_orders_user_created")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !tag.Index || tag.IndexName != "idx_orders_user_created" {
		t.Errorf("composite index parse failed: %+v", tag)
	}
}

func TestParseTag_Invalid(t *testing.T) {
	_, err := parser.ParseTag("col,size:not_a_number")
	if err == nil {
		t.Fatal("expected error for invalid size")
	}
	_, err = parser.ParseTag("col,unknown_thing")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}
