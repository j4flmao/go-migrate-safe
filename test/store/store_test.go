package store_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/store"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListFiles_SortsAndParses(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0002_alter_users.up.sql", "-- Version: 2\n-- Name: alter_users\n-- Direction: UP\n\nALTER TABLE users ADD COLUMN x INT;\n")
	writeFile(t, dir, "0002_alter_users.down.sql", "-- Version: 2\n-- Name: alter_users\n-- Direction: DOWN\n\nALTER TABLE users DROP COLUMN x;\n")
	writeFile(t, dir, "0001_init.up.sql", "-- Version: 1\n-- Name: init\n-- Direction: UP\n\nCREATE TABLE users (id INT);\n")
	writeFile(t, dir, "0001_init.down.sql", "-- Version: 1\n-- Name: init\n-- Direction: DOWN\n\nDROP TABLE users;\n")
	writeFile(t, dir, "ignored.txt", "ignored")

	files, err := store.ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("got %d files, want 4: %+v", len(files), files)
	}
	if files[0].Version != 1 || files[0].Direction != "down" {
		t.Errorf("first file = %+v", files[0])
	}
	if files[3].Version != 2 || files[3].Direction != "up" {
		t.Errorf("last file = %+v", files[3])
	}
	if files[3].Header.Name != "alter_users" {
		t.Errorf("header parse failed: %+v", files[3].Header)
	}
}

func TestNextVersion_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	v, err := store.NextVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Errorf("next = %d, want 1", v)
	}
}

func TestNextVersion_AfterFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0005_x.up.sql", "")
	writeFile(t, dir, "0005_x.down.sql", "")
	v, err := store.NextVersion(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v != 6 {
		t.Errorf("next = %d, want 6", v)
	}
}

func TestPendingFiles_RespectsHistory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0001_a.up.sql", "")
	writeFile(t, dir, "0002_b.up.sql", "")
	writeFile(t, dir, "0003_c.up.sql", "")
	hist := []driver.MigrationRecord{
		{Version: 1, Direction: "up", Status: "applied"},
	}
	pending, err := store.PendingFiles(dir, hist)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Fatalf("got %d pending, want 2", len(pending))
	}
	if pending[0].Version != 2 || pending[1].Version != 3 {
		t.Errorf("pending = %+v", pending)
	}
}

func TestBuildStatus(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "0001_init.up.sql", "")
	writeFile(t, dir, "0001_init.down.sql", "")
	writeFile(t, dir, "0002_alter.up.sql", "")
	writeFile(t, dir, "0002_alter.down.sql", "")
	hist := []driver.MigrationRecord{
		{Version: 1, Direction: "up", Status: "applied", AppliedAt: "2024-01-01T00:00:00Z"},
	}
	st, err := store.BuildStatus(context.Background(), dir, hist)
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 {
		t.Fatalf("got %d statuses, want 2", len(st))
	}
	if !st[0].Applied || st[1].Applied {
		t.Errorf("expected v1 applied, v2 pending; got %+v", st)
	}
}

func TestChecksumBody_DeterministicAndDifferent(t *testing.T) {
	a := store.ChecksumBody("hello")
	b := store.ChecksumBody("hello")
	c := store.ChecksumBody("world")
	if a != b {
		t.Fatal("checksum should be deterministic")
	}
	if a == c {
		t.Fatal("checksum should differ for different input")
	}
}
