package studio_test

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/j4flmao/go-migrate-safe/driver/sqlite"
	"github.com/j4flmao/go-migrate-safe/studio"
	_ "github.com/mattn/go-sqlite3"
)

func TestStudio_API(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Setup table
	_, _ = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	_, _ = db.Exec("INSERT INTO users (id, name) VALUES (1, 'Alice'), (2, 'Bob')")

	drv := sqlite.New(db)
	srv, err := studio.New(studio.Options{
		DB:         db,
		DriverName: "sqlite",
		Driver:     drv,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	addr := "127.0.0.1:4499"
	go func() {
		_ = srv.Listen(addr)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	baseURL := "http://" + addr

	t.Run("Info", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/info")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var data map[string]any
		json.NewDecoder(resp.Body).Decode(&data)
		if data["driver"] != "sqlite" {
			t.Errorf("expected sqlite, got %v", data["driver"])
		}
	})

	t.Run("Tables", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/tables")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var data []map[string]any
		json.NewDecoder(resp.Body).Decode(&data)

		found := false
		for _, tbl := range data {
			if tbl["name"] == "users" {
				found = true
				if tbl["rowCount"].(float64) != 2 {
					t.Errorf("expected 2 rows, got %v", tbl["rowCount"])
				}
			}
		}
		if !found {
			t.Error("users table not found in API response")
		}
	})

	t.Run("TableData", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/table/users")
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		var data map[string]any
		json.NewDecoder(resp.Body).Decode(&data)

		if data["name"] != "users" {
			t.Errorf("expected table name users, got %v", data["name"])
		}
		rows := data["rows"].([]any)
		if len(rows) != 2 {
			t.Errorf("expected 2 rows, got %d", len(rows))
		}
	})

	t.Run("Insert", func(t *testing.T) {
		body := `{"values": {"id": 3, "name": "Charlie"}}`
		resp, err := http.Post(baseURL+"/api/table/users/insert", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(b))
		}

		// Verify in DB
		var count int
		db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if count != 3 {
			t.Errorf("expected 3 rows after insert, got %d", count)
		}
	})

	t.Run("Update", func(t *testing.T) {
		body := `{"pk": {"id": 1}, "values": {"name": "Alice Updated"}}`
		resp, err := http.Post(baseURL+"/api/table/users/update", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(b))
		}

		var name string
		db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
		if name != "Alice Updated" {
			t.Errorf("expected Alice Updated, got %s", name)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		body := `{"pks": [{"id": 2}]}`
		resp, err := http.Post(baseURL+"/api/table/users/delete", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(b))
		}

		var count int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE id = 2").Scan(&count)
		if count != 0 {
			t.Error("record id=2 still exists after delete")
		}
	})
}

// Add ServeHTTP to Server for easier testing if it's not already there.
// Wait, I should check if Server implements ServeHTTP.
// I saw mux := http.NewServeMux() in Listen but it doesn't seem to export it.
