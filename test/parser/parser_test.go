package parser_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/parser"
)

type User struct {
	ID        int64     `db:"id,pk,autoincrement"`
	Email     string    `db:"email,not null,unique,size:128"`
	Name      *string   `db:"name"`
	CreatedAt time.Time `db:"created_at,not null,default:NOW()"`
	Internal  string    `db:"-"`
}

type Order struct {
	ID     int64           `db:"id,pk,autoincrement"`
	UserID int64           `db:"user_id,not null,fk:users(id),index"`
	Status string          `db:"status,not null,default:'pending'"`
	Notes  sql.NullString  `db:"notes"`
	Amount float64         `db:"amount,not null,type:DECIMAL(12,4)"`
}

func TestParser_AllTagOptions_ProducesCorrectSchemaModel(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(User{}, Order{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, ok := sm.Tables["users"]; !ok {
		t.Fatalf("users table missing: %+v", sm.Tables)
	}
	users := sm.Tables["users"]
	if c, ok := users.Columns["id"]; !ok || !c.IsPK || !c.AutoIncrement {
		t.Errorf("users.id wrong: %+v", c)
	}
	if c := users.Columns["email"]; c.Nullable {
		t.Errorf("email should be NOT NULL: %+v", c)
	}
	if c := users.Columns["email"]; c.Size == nil || *c.Size != 128 || c.SQLType != "VARCHAR(128)" {
		t.Errorf("email should be VARCHAR(128): %+v (size=%v)", c.SQLType, c.Size)
	}
	if c := users.Columns["name"]; !c.Nullable {
		t.Errorf("name should be nullable (pointer): %+v", c)
	}
	if c := users.Columns["created_at"]; c.Default == nil || *c.Default != "NOW()" {
		t.Errorf("created_at default wrong: %+v", c.Default)
	}
	if _, exists := users.Columns["internal"]; exists {
		t.Errorf("internal field should be ignored")
	}
}

func TestParser_PointerAndNullableInferred(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(Order{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	notes := sm.Tables["orders"].Columns["notes"]
	if !notes.Nullable {
		t.Errorf("sql.NullString should be nullable: %+v", notes)
	}
	if amount := sm.Tables["orders"].Columns["amount"]; amount.SQLType != "DECIMAL(12,4)" {
		t.Errorf("amount type override = %q", amount.SQLType)
	}
}

func TestParser_FKConstraintCreated(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(Order{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	orders := sm.Tables["orders"]
	if c, ok := orders.Constraints["fk_orders_users"]; !ok || c.Kind != migrate.ConstraintForeignKey {
		t.Errorf("FK constraint missing or wrong: %+v", orders.Constraints)
	}
	if _, ok := orders.Indexes["idx_orders_user_id"]; !ok {
		t.Errorf("idx_orders_user_id missing")
	}
}

func TestParser_NoModels_Errors(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	_, err := p.Parse()
	if err == nil {
		t.Fatal("expected ErrNoModels")
	}
}

type Visit struct {
	UserID    int64 `db:"user_id,not null,index:idx_visits_user_created"`
	CreatedAt int64 `db:"created_at,not null,index:idx_visits_user_created"`
}

func TestParser_CompositeIndex(t *testing.T) {
	p := parser.New(parser.DialectPostgres, "public")
	sm, err := p.Parse(Visit{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tbl, ok := sm.Tables["visits"]
	if !ok {
		t.Fatalf("visits table missing: %+v", sm.Tables)
	}
	idx, ok := tbl.Indexes["idx_visits_user_created"]
	if !ok {
		t.Fatalf("composite index missing: %+v", tbl.Indexes)
	}
	if len(idx.Columns) != 2 {
		t.Errorf("composite index should have 2 cols, got %v", idx.Columns)
	}
}
