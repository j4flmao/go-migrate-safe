// Package driver provides database backend implementations.
//
// The Driver interface is defined in the migrate package.
// Bundled drivers live under driver/<name>/ (postgres, mysql, sqlite, memory).
// Users import exactly one of them next to the migrate package.
package driver

import "github.com/j4flmao/go-migrate-safe/migrate"

// Driver aliases migrate.Driver for convenience.
type Driver = migrate.Driver

// Tx aliases migrate.Tx for convenience.
type Tx = migrate.Tx

// MigrationRecord aliases migrate.MigrationRecord for convenience.
type MigrationRecord = migrate.MigrationRecord
