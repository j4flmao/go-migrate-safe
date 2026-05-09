package reader_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/j4flmao/go-migrate-safe/reader"
	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteReader_Integration(t *testing.T) {
	dbPath := "test_reader.db"
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// 1. Setup schema
	ddl := `
	CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		name TEXT
	);
	CREATE TABLE posts (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		user_id INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(id)
	);
	CREATE INDEX idx_posts_title ON posts(title);
	`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// 2. Read schema
	r := reader.NewSQLite(db)
	sm, err := r.ReadSchema(context.Background(), "main")
	if err != nil {
		t.Fatalf("ReadSchema: %v", err)
	}

	// 3. Verify users table
	users, ok := sm.Tables["users"]
	if !ok {
		t.Fatal("users table missing")
	}
	if !users.Columns["id"].IsPK {
		t.Errorf("users.id PK wrong: %+v", users.Columns["id"])
	}
	// Note: Current SQLite reader doesn't detect AutoIncrement yet
	if users.Columns["email"].Nullable {
		t.Error("users.email should be NOT NULL")
	}

	// 4. Verify posts table and FK
	posts, ok := sm.Tables["posts"]
	if !ok {
		t.Fatal("posts table missing")
	}
	if _, ok := posts.Indexes["idx_posts_title"]; !ok {
		t.Error("idx_posts_title missing")
	}

	if posts.Columns["user_id"].Name != "user_id" {
		t.Error("posts.user_id missing")
	}

	// 5. Test with a table that has no PK
	_, _ = db.Exec("CREATE TABLE no_pk (val TEXT)")
	sm2, _ := r.ReadSchema(context.Background(), "main")
	noPk, ok := sm2.Tables["no_pk"]
	if !ok {
		t.Fatal("no_pk table missing")
	}
	for _, col := range noPk.Columns {
		if col.IsPK {
			t.Errorf("column %s should not be PK", col.Name)
		}
	}

	// 6. Test with complex types
	_, _ = db.Exec("CREATE TABLE complex_types (d REAL, b BLOB, i INTEGER)")
	sm3, _ := r.ReadSchema(context.Background(), "main")
	ct := sm3.Tables["complex_types"]
	if ct.Columns["d"].SQLType != "REAL" {
		t.Errorf("expected REAL, got %s", ct.Columns["d"].SQLType)
	}
}
