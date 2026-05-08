package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/j4flmao/go-migrate-safe/driver"
	"github.com/j4flmao/go-migrate-safe/driver/memory"
	"github.com/j4flmao/go-migrate-safe/driver/mongodb"
	"github.com/j4flmao/go-migrate-safe/driver/mssql"
	"github.com/j4flmao/go-migrate-safe/driver/mysql"
	"github.com/j4flmao/go-migrate-safe/driver/postgres"
	"github.com/j4flmao/go-migrate-safe/driver/sqlite"
	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/orchestrator"
)

const usage = `gms — go-migrate-safe CLI

Usage:
  gms <command> [flags]

Commands:
  generate    Generate migration files from Go struct models
  apply       Apply pending migrations
  status      Show current migration status
  history     Show migration history
  validate    Validate all migration files
  rollback    Rollback last N migrations
  diff        Show what would change (no files written)

Flags:
  --driver NAME     DB driver: postgres | mysql | sqlite | mssql | mongodb | memory (auto-detected from DATABASE_URL)
  --dir PATH        Migrations directory (default ./migrations)
  --schema NAME     Schema name (default public; for MySQL this is database name)
  --name NAME       Override generated migration name
  --allow-drop-table     Permit DROP TABLE
  --allow-drop-column    Permit DROP COLUMN
  --allow-type-change MODE  none | safe | any
  --dry-run              Apply without committing
  --no-rollback-required Skip missing .down.sql check

Environment:
  DATABASE_URL       Database connection string
                       MySQL:    root@tcp(127.0.0.1:3306)/dbname
                       Postgres: postgres://user:pass@localhost/dbname
                       SQLite:   /path/to/db.sqlite
                       MSSQL:    sqlserver://user:pass@localhost/dbname
                       MongoDB:  mongodb://localhost:27017/dbname
  MYSQL_DSN          MySQL DSN override
  PGSQL_DSN          Postgres DSN override
  SQLITE_PATH        SQLite file path override
  MSSQL_DSN          MSSQL DSN override
  MONGODB_URI        MongoDB URI override

Examples:
  gms status                          # memory driver (no DB needed for file-only checks)
  gms generate --driver mysql         # needs models registered in custom main.go
  gms apply                           # apply pending migrations
`

var modelRegistry []any

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	driverName := fs.String("driver", "", "")
	dir := fs.String("dir", "./migrations", "")
	schema := fs.String("schema", "public", "")
	name := fs.String("name", "", "")
	allowDropTable := fs.Bool("allow-drop-table", false, "")
	allowDropColumn := fs.Bool("allow-drop-column", false, "")
	allowTypeChange := fs.String("allow-type-change", "none", "")
	dryRun := fs.Bool("dry-run", false, "")
	noRollback := fs.Bool("no-rollback-required", false, "")
	if err := fs.Parse(os.Args[2:]); err != nil {
		exitErr(err, 1)
	}

	if *driverName == "" {
		*driverName = detectDriver()
	}

	opts := []migrate.Option{
		migrate.WithOutputDir(*dir),
		migrate.WithSchema(*schema),
		migrate.WithDriver(*driverName),
	}
	if *allowDropTable {
		opts = append(opts, migrate.WithAllowDropTable())
	}
	if *allowDropColumn {
		opts = append(opts, migrate.WithAllowDropColumn())
	}
	switch *allowTypeChange {
	case "safe":
		opts = append(opts, migrate.WithAllowTypeChange(migrate.TypeChangeSafe))
	case "any":
		opts = append(opts, migrate.WithAllowTypeChange(migrate.TypeChangeAny))
	}
	if *noRollback {
		opts = append(opts, migrate.WithNoRollbackRequired())
	}
	if len(modelRegistry) > 0 {
		opts = append(opts, migrate.WithModels(modelRegistry...))
	}
	m, err := migrate.New(opts...)
	if err != nil {
		exitErr(err, 1)
	}

	d, err := openDriver(*driverName)
	if err != nil {
		exitErr(err, 1)
	}
	defer d.Close()

	intent := cmd
	if cmd == "apply" && *dryRun {
		intent = "apply-dry-run"
	}

	ctx := context.Background()
	res, err := orchestrator.Run(ctx, intent, orchestrator.Options{
		Migrator:     m,
		DBDriver:     d,
		OutputDir:    *dir,
		DialectName:  *driverName,
		NameOverride: *name,
	})
	if err != nil {
		exitErr(err, classifyExit(res, err))
	}
	if res.Explain != "" {
		fmt.Print(res.Explain)
	}
	if len(res.Warnings) > 0 {
		fmt.Fprintln(os.Stderr, "Warnings:")
		for _, w := range res.Warnings {
			fmt.Fprintf(os.Stderr, "  - [%s] %s\n", w.Code, w.Message)
		}
	}
	if len(res.Errors) > 0 {
		fmt.Fprintln(os.Stderr, "Errors:")
		for _, e := range res.Errors {
			fmt.Fprintf(os.Stderr, "  - [%s] %s\n", e.Code, e.Message)
			if e.Suggestion != "" {
				fmt.Fprintf(os.Stderr, "      → %s\n", e.Suggestion)
			}
		}
		os.Exit(classifyExit(res, nil))
	}
	if len(res.GeneratedFiles) > 0 {
		fmt.Fprintln(os.Stderr, "Generated:")
		for _, f := range res.GeneratedFiles {
			fmt.Fprintf(os.Stderr, "  %s\n", f)
		}
	}
}

func detectDriver() string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return "memory"
	}
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return "postgres"
	}
	if strings.Contains(dsn, "@tcp(") || strings.Contains(dsn, "@unix(") {
		return "mysql"
	}
	if strings.HasPrefix(dsn, "sqlserver://") || strings.HasPrefix(dsn, "mssql://") {
		return "mssql"
	}
	if strings.HasPrefix(dsn, "mongodb://") || strings.HasPrefix(dsn, "mongodb+srv://") {
		return "mongodb"
	}
	return "sqlite"
}

func openDriver(name string) (driver.Driver, error) {
	switch name {
	case "memory", "":
		return memory.New(), nil

	case "mysql":
		dsn := envOr("MYSQL_DSN", os.Getenv("DATABASE_URL"))
		if dsn == "" {
			return nil, fmt.Errorf("mysql: set DATABASE_URL or MYSQL_DSN")
		}
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("mysql: %w", err)
		}
		if err := db.PingContext(context.Background()); err != nil {
			db.Close()
			return nil, fmt.Errorf("mysql: %w", err)
		}
		return mysql.New(db), nil

	case "postgres":
		dsn := envOr("PGSQL_DSN", os.Getenv("DATABASE_URL"))
		if dsn == "" {
			return nil, fmt.Errorf("postgres: set DATABASE_URL or PGSQL_DSN")
		}
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return nil, fmt.Errorf("postgres: %w", err)
		}
		if err := db.PingContext(context.Background()); err != nil {
			db.Close()
			return nil, fmt.Errorf("postgres: %w", err)
		}
		return postgres.New(db), nil

	case "sqlite":
		dsn := envOr("SQLITE_PATH", os.Getenv("DATABASE_URL"))
		if dsn == "" {
			return nil, fmt.Errorf("sqlite: set DATABASE_URL or SQLITE_PATH")
		}
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, fmt.Errorf("sqlite: %w", err)
		}
		if err := db.PingContext(context.Background()); err != nil {
			db.Close()
			return nil, fmt.Errorf("sqlite: %w", err)
		}
		return sqlite.New(db), nil

	case "mssql":
		dsn := envOr("MSSQL_DSN", os.Getenv("DATABASE_URL"))
		if dsn == "" {
			return nil, fmt.Errorf("mssql: set DATABASE_URL or MSSQL_DSN")
		}
		db, err := sql.Open("sqlserver", dsn)
		if err != nil {
			return nil, fmt.Errorf("mssql: %w", err)
		}
		if err := db.PingContext(context.Background()); err != nil {
			db.Close()
			return nil, fmt.Errorf("mssql: %w", err)
		}
		return mssql.New(db), nil

	case "mongodb":
		uri := envOr("MONGODB_URI", os.Getenv("DATABASE_URL"))
		if uri == "" {
			return nil, fmt.Errorf("mongodb: set DATABASE_URL or MONGODB_URI")
		}
		dbName := extractMongoDBName(uri)
		ctx := context.Background()
		drv, err := mongodb.New(ctx, uri, dbName)
		if err != nil {
			return nil, fmt.Errorf("mongodb: %w", err)
		}
		return drv, nil

	default:
		return nil, fmt.Errorf("%w: %q", migrate.ErrUnsupportedDriver, name)
	}
}

func classifyExit(res *orchestrator.Result, err error) int {
	if err != nil {
		if errors.Is(err, migrate.ErrLockTimeout) {
			return 1
		}
		if errors.Is(err, migrate.ErrChecksumMismatch) {
			return 4
		}
		return 1
	}
	if res != nil {
		for _, e := range res.Errors {
			switch e.Code {
			case "ErrMigrationConflict":
				return 2
			case "ErrDestructiveOp":
				return 3
			default:
				return 4
			}
		}
	}
	return 0
}

func exitErr(err error, code int) {
	fmt.Fprintf(os.Stderr, "gms: %v\n", err)
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func extractMongoDBName(uri string) string {
	// mongodb://host:port/dbname or mongodb+srv://host/dbname
	if idx := strings.LastIndexByte(uri, '/'); idx >= 0 {
		name := uri[idx+1:]
		if q := strings.IndexByte(name, '?'); q >= 0 {
			name = name[:q]
		}
		if name != "" {
			return name
		}
	}
	return "migrations"
}
