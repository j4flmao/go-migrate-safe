package parser_test

import (
	"testing"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/parser"
)

type CustomTable struct {
	_    struct{} `db:"table:custom_name"`
	ID   int64    `db:"id,pk"`
	Data string   `db:"data"`
}

func TestParser_TableRename(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(CustomTable{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := sm.Tables["custom_name"]; !ok {
		t.Errorf("expected table custom_name, got %+v", sm.Tables)
	}
}

type CompositePK struct {
	TenantID int64  `db:"tenant_id,pk"`
	UserID   int64  `db:"user_id,pk"`
	Role     string `db:"role"`
}

func TestParser_CompositePK(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(CompositePK{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tbl := sm.Tables["composite_pks"]
	pk, ok := tbl.Constraints["pk_composite_pks"]
	if !ok || pk.Kind != migrate.ConstraintPrimaryKey {
		t.Fatalf("PK constraint missing or wrong type")
	}
	if len(pk.Columns) != 2 {
		t.Errorf("expected 2 columns in PK, got %v", pk.Columns)
	}
}

type UniqueConstraints struct {
	Email    string `db:"email,unique"`
	Username string `db:"username,unique:uniq_user"`
	OrgID    int64  `db:"org_id,unique:uniq_org_user"`
	Code     string `db:"code,unique:uniq_org_user"`
}

func TestParser_UniqueConstraints(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(UniqueConstraints{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tbl := sm.Tables["unique_constraintses"]

	// Default unique name
	if _, ok := tbl.Indexes["uniq_unique_constraintses_email"]; !ok {
		t.Errorf("missing default unique index")
	}

	// Named unique
	if _, ok := tbl.Indexes["uniq_user"]; !ok {
		t.Errorf("missing named unique index")
	}

	// Composite unique
	idx, ok := tbl.Indexes["uniq_org_user"]
	if !ok {
		t.Fatalf("missing composite unique index 'uniq_org_user'")
	}
	if len(idx.Columns) != 2 {
		t.Errorf("expected 2 columns in unique index, got %v", idx.Columns)
	}
}

type DataTypeEdgeCases struct {
	Blob   []byte  `db:"blob"`
	Price  float32 `db:"price"`
	Rating float64 `db:"rating"`
	Active bool    `db:"active"`
}

func TestParser_DataTypes(t *testing.T) {
	dialects := []parser.Dialect{parser.DialectPostgres, parser.DialectMySQL, parser.DialectSQLite}
	for _, d := range dialects {
		t.Run(string(d), func(t *testing.T) {
			p := parser.New(d, "public")
			sm, err := p.Parse(DataTypeEdgeCases{})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			tbl := sm.Tables["data_type_edge_caseses"]
			if c := tbl.Columns["blob"]; c.SQLType == "" {
				t.Errorf("missing type for blob")
			}
			if c := tbl.Columns["active"]; c.SQLType == "" {
				t.Errorf("missing type for active")
			}
		})
	}
}
