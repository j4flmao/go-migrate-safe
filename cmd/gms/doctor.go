package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/store"
	"github.com/j4flmao/go-migrate-safe/validator"
)

func runDoctor(driverName string, dir string) {
	fmt.Printf("🩺 GMS Doctor — Diagnosing your database and migration health...\n\n")

	ctx := context.Background()

	// 1. Connection Check
	fmt.Printf("1. Database Connection: ")
	d, err := openDriver(driverName)
	if err != nil {
		fmt.Printf("❌ FAILED\n   Error: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	dbVer, err := d.DatabaseVersion(ctx)
	if err != nil {
		fmt.Printf("❌ FAILED (Ping/Version)\n   Error: %v\n", err)
	} else {
		fmt.Printf("✅ OK (%s %s)\n", driverName, dbVer)
	}

	// 2. Permissions Check (Can we create/read/insert?)
	fmt.Printf("2. Database Permissions: ")
	err = d.EnsureHistoryTable(ctx)
	if err != nil {
		fmt.Printf("❌ FAILED (Cannot manage history table)\n   Error: %v\n", err)
	} else {
		fmt.Printf("✅ OK (Can manage _migrate_history)\n")
	}

	// 3. Migration Files Check
	fmt.Printf("3. Migration Files: ")
	files, err := store.ListFiles(dir)
	if err != nil {
		fmt.Printf("❌ FAILED (Cannot read directory %q)\n   Error: %v\n", dir, err)
	} else {
		fmt.Printf("✅ OK (%d files found in %s)\n", len(files), dir)
	}

	// 4. Migration Integrity Check (Conflicts/Checksums)
	fmt.Printf("4. Migration Integrity: ")
	hist, _ := d.LoadHistory(ctx)
	// We use a default safety option for validation
	v := validator.New(migrate.SafetyOptions{})
	report := v.FullValidate(ctx, files, hist)
	if len(report.Errors) > 0 {
		fmt.Printf("❌ FAILED (%d issues found)\n", len(report.Errors))
		for _, e := range report.Errors {
			fmt.Printf("   - [%s] %s\n", e.Code, e.Message)
		}
	} else {
		fmt.Printf("✅ OK\n")
	}

	// 5. Lock Check
	fmt.Printf("5. Lock Status: ")
	lockStart := time.Now()
	err = d.AcquireLock(ctx)
	if err != nil {
		fmt.Printf("⚠️ LOCKED (Database is currently locked by another process or stale lock)\n   Error: %v\n", err)
	} else {
		_ = d.ReleaseLock(ctx)
		fmt.Printf("✅ OK (Lock acquired in %v)\n", time.Since(lockStart).Round(time.Millisecond))
	}

	fmt.Printf("\n✨ Diagnosis complete. ")
	if len(report.Errors) > 0 {
		fmt.Printf("Please fix the issues above before running migrations.\n")
	} else {
		fmt.Printf("Your environment looks healthy!\n")
	}
}
