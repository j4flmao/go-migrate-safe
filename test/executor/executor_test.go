package executor_test

import (
	"context"
	"testing"

	"github.com/j4flmao/go-migrate-safe/driver/memory"
	"github.com/j4flmao/go-migrate-safe/executor"
	"github.com/j4flmao/go-migrate-safe/store"
)

func TestApply_EmptyFiles(t *testing.T) {
	d := memory.New()
	ex := executor.New(d)
	res, err := ex.Apply(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Applied) != 0 {
		t.Errorf("expected 0 applied, got %d", len(res.Applied))
	}
}

func TestApply_SingleUp(t *testing.T) {
	d := memory.New()
	ex := executor.New(d)
	files := []store.File{
		{Path: "0001_init.up.sql", Version: 1, Name: "init", Direction: "up",
			Body: "CREATE TABLE users (id BIGINT);", Checksum: "abc"},
	}
	res, err := ex.Apply(context.Background(), files)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(res.Applied) != 1 {
		t.Fatalf("applied %d, want 1", len(res.Applied))
	}
	if res.Failed != nil {
		t.Errorf("failed = %v", res.Failed)
	}
}

func TestApply_DryRun_SkipsExecution(t *testing.T) {
	d := memory.New()
	ex := executor.New(d)
	ex.DryRun = true
	files := []store.File{
		{Path: "0001_init.up.sql", Version: 1, Name: "init", Direction: "up",
			Body: "CREATE TABLE users (id BIGINT);", Checksum: "abc"},
	}
	_, err := ex.Apply(context.Background(), files)
	if err != nil {
		t.Fatalf("dry-run apply: %v", err)
	}
	if len(d.ExecLog) != 0 {
		t.Errorf("dry-run should not execute SQL, got %v", d.ExecLog)
	}
}

func TestApply_Rollback(t *testing.T) {
	d := memory.New()

	// First apply up
	ex := executor.New(d)
	upFiles := []store.File{
		{Path: "0001_init.up.sql", Version: 1, Name: "init", Direction: "up",
			Body: "CREATE TABLE users (id BIGINT);", Checksum: "abc"},
	}
	_, err := ex.Apply(context.Background(), upFiles)
	if err != nil {
		t.Fatalf("apply up: %v", err)
	}

	// Now rollback using down file
	downFiles := []store.File{
		{Path: "0001_init.down.sql", Version: 1, Name: "init", Direction: "down",
			Body: "DROP TABLE IF EXISTS users;", Checksum: "def"},
	}
	res, err := ex.Rollback(context.Background(), downFiles)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if len(res.Applied) != 1 {
		t.Errorf("rolled back %d, want 1", len(res.Applied))
	}
}

func TestSplitStatements_Basic(t *testing.T) {
	body := "CREATE TABLE users (id BIGINT);\nALTER TABLE users ADD COLUMN name TEXT;\nINSERT INTO users VALUES (1);"
	stmts := executor.SplitStatements(body)
	if len(stmts) != 3 {
		t.Fatalf("got %d statements, want 3: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_Comments(t *testing.T) {
	body := "-- this is a comment\nCREATE TABLE users (id BIGINT);\n-- another comment\nALTER TABLE users ADD COLUMN name TEXT;"
	stmts := executor.SplitStatements(body)
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2: %v", len(stmts), stmts)
	}
}

func TestSplitStatements_MultiLine(t *testing.T) {
	body := "CREATE TABLE users (\n  id BIGINT PRIMARY KEY\n);\nALTER TABLE users ADD COLUMN name TEXT;"
	stmts := executor.SplitStatements(body)
	if len(stmts) != 2 {
		t.Fatalf("got %d statements, want 2: %v", len(stmts), stmts)
	}
}

func TestApply_RecordsHistoryOnSuccess(t *testing.T) {
	d := memory.New()
	ex := executor.New(d)
	files := []store.File{
		{Path: "0001_init.up.sql", Version: 1, Name: "init", Direction: "up",
			Body: "CREATE TABLE users (id BIGINT);", Checksum: "abc"},
	}
	_, err := ex.Apply(context.Background(), files)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	hist, _ := d.LoadHistory(context.Background())
	if len(hist) != 1 {
		t.Fatalf("history len = %d, want 1", len(hist))
	}
	if hist[0].Status != "applied" {
		t.Errorf("status = %q, want applied", hist[0].Status)
	}
	if hist[0].Version != 1 {
		t.Errorf("version = %d, want 1", hist[0].Version)
	}
}

func TestApply_RecordsFailureOnError(t *testing.T) {
	d := memory.New()
	ex := executor.New(d)
	files := []store.File{
		{Path: "0001_init.up.sql", Version: 1, Name: "init", Direction: "up",
			Body: "INVALID SQL;", Checksum: "abc"},
	}
	_, err := ex.Apply(context.Background(), files)
	if err == nil {
		t.Fatal("expected error for invalid SQL")
	}
	hist, _ := d.LoadHistory(context.Background())
	if len(hist) != 1 {
		t.Fatalf("history len = %d, want 1", len(hist))
	}
	if hist[0].Status != "failed" {
		t.Errorf("status = %q, want failed", hist[0].Status)
	}
	if hist[0].ErrorMessage == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRollback_UpdatesHistoryRecord(t *testing.T) {
	d := memory.New()
	ex := executor.New(d)
	upFiles := []store.File{
		{Path: "0001_init.up.sql", Version: 1, Name: "init", Direction: "up",
			Body: "CREATE TABLE users (id BIGINT);", Checksum: "abc"},
	}
	ex.Apply(context.Background(), upFiles)
	downFiles := []store.File{
		{Path: "0001_init.down.sql", Version: 1, Name: "init", Direction: "down",
			Body: "DROP TABLE IF EXISTS users;", Checksum: "def"},
	}
	_, err := ex.Rollback(context.Background(), downFiles)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	hist, _ := d.LoadHistory(context.Background())
	rollbackFound := false
	for _, h := range hist {
		if h.Direction == "down" && h.Status == "rolled_back" {
			rollbackFound = true
			break
		}
	}
	if !rollbackFound {
		t.Errorf("expected rolled_back record, got %+v", hist)
	}
}
