package parser_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/parser"
)

type DetailedAttributes struct {
	Name      string  `db:"name,size:100"`
	Price     float64 `db:"price,precision:10,scale:2"`
	Status    string  `db:"status,default:'pending'"`
	AutoTime  int64   `db:"auto_time,default:now()"`
}

func TestParser_DetailedAttributes(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(DetailedAttributes{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tbl := sm.Tables["detailed_attributeses"]

	// Size
	nameCol := tbl.Columns["name"]
	if nameCol.Size == nil || *nameCol.Size != 100 {
		t.Errorf("expected size 100, got %v", nameCol.Size)
	}
	if nameCol.SQLType != "VARCHAR(100)" {
		t.Errorf("expected VARCHAR(100), got %s", nameCol.SQLType)
	}

	// Precision & Scale
	priceCol := tbl.Columns["price"]
	if priceCol.Precision == nil || *priceCol.Precision != 10 {
		t.Errorf("expected precision 10, got %v", priceCol.Precision)
	}
	if priceCol.Scale == nil || *priceCol.Scale != 2 {
		t.Errorf("expected scale 2, got %v", priceCol.Scale)
	}

	// Default
	statusCol := tbl.Columns["status"]
	if statusCol.Default == nil || *statusCol.Default != "'pending'" {
		t.Errorf("expected default 'pending', got %v", statusCol.Default)
	}

	timeCol := tbl.Columns["auto_time"]
	if timeCol.Default == nil || *timeCol.Default != "now()" {
		t.Errorf("expected default now(), got %v", timeCol.Default)
	}
}

type PointerTypes struct {
	Bio    *string `db:"bio"`
	Age    *int    `db:"age"`
	Active *bool   `db:"active"`
}

func TestParser_PointerTypes(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, _ := p.Parse(PointerTypes{})
	tbl := sm.Tables["pointer_typeses"]

	for _, col := range tbl.Columns {
		if !col.Nullable {
			t.Errorf("column %s should be nullable", col.Name)
		}
	}
}
