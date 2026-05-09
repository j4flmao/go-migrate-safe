package studio_test

import (
	"bytes"
	"database/sql"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/driver/sqlite"
	"github.com/j4flmao/go-migrate-safe/studio"
	_ "github.com/mattn/go-sqlite3"
)

func TestStudio_Security_SQLInjection(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	_, _ = db.Exec("INSERT INTO users (id, name) VALUES (1, 'Alice')")

	drv := sqlite.New(db)
	srv, _ := studio.New(studio.Options{
		DB:         db,
		DriverName: "sqlite",
		Driver:     drv,
	})

	addr := "127.0.0.1:4500"
	go func() { _ = srv.Listen(addr) }()
	time.Sleep(100 * time.Millisecond)
	baseURL := "http://" + addr

	t.Run("Injection_In_TableName", func(t *testing.T) {
		// Attempting to access a non-existent table with injection attempt
		resp, _ := http.Get(baseURL + "/api/table/users;DROP+TABLE+users;--")
		if resp != nil && resp.StatusCode == http.StatusOK {
			t.Error("should not return 200 for injected table name")
		}

		// Verify table still exists
		var name string
		err := db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
		if err != nil {
			t.Fatalf("Table might have been dropped or data lost: %v", err)
		}
	})

	t.Run("Injection_In_InsertValues", func(t *testing.T) {
		body := `{"values": {"id": 99, "name": "Hacker', 'x'); DROP TABLE users; --"}}`
		resp, _ := http.Post(baseURL+"/api/table/users/insert", "application/json", strings.NewReader(body))

		if resp != nil {
			resp.Body.Close()
		}

		// Verify table still exists
		var name string
		err := db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
		if err != nil {
			t.Fatalf("Table dropped via Insert injection! %v", err)
		}
	})
}

func TestStudio_EdgeCases_InvalidJSON(t *testing.T) {
	db, _ := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")

	drv := sqlite.New(db)
	srv, _ := studio.New(studio.Options{DB: db, DriverName: "sqlite", Driver: drv})

	addr := "127.0.0.1:4501"
	go func() { _ = srv.Listen(addr) }()
	time.Sleep(100 * time.Millisecond)
	baseURL := "http://" + addr

	t.Run("Invalid_JSON_Body", func(t *testing.T) {
		resp, _ := http.Post(baseURL+"/api/table/users/insert", "application/json", bytes.NewBufferString("{invalid json}"))
		if resp != nil && resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
		}
	})
}
