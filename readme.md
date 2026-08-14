# go-migrate-safe

Type-safe database migration library for Go — with auto struct-diff, dry-run, conflict detection, and rollback planning.

## Features

- **Struct-first migrations**: Your Go types are the source of truth
- **Auto-generated SQL**: No manual SQL writing for common changes
- **Database agnostic**: Supports PostgreSQL, MySQL, SQLite, SQL Server, and MongoDB
- **Safety by default**: Destructive operations require explicit opt-in
- **Rollback planning**: Every migration includes a rollback plan
- **Dry run mode**: Test migrations before applying to production
- **GMS Studio**: Built-in modern web UI for database browsing and management (CRUD)

## Installation

Install the CLI:
```bash
go install github.com/j4flmao/go-migrate-safe/cmd/gms@latest
```

Add as a dependency to your project:
```bash
go get github.com/j4flmao/go-migrate-safe
```

## Quick Start

### 1. Define your models

```go
package models

import "time"

type User struct {
	ID        int64     `db:"id,pk,autoincrement"`
	Email     string    `db:"email,unique,not null"`
	Password  string    `db:"password,not null"`
	CreatedAt time.Time `db:"created_at,not null,default:now()"`
}

type Product struct {
	ID          int64   `db:"id,pk,autoincrement"`
	Name        string  `db:"name,not null"`
	Description string  `db:"description,not null"`
	Price       float64 `db:"price,not null"`
}
```

### 2. Create a migration wrapper

Create `cmd/gms/main.go` in your project:

```go
package main

import (
	"log"
	"os"

	"github.com/j4flmao/go-migrate-safe/migrate"
	"github.com/j4flmao/go-migrate-safe/orchestrator"
	_ "github.com/lib/pq"
	// Import your models package
	"github.com/yourusername/yourproject/models"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	m, err := migrate.New(
		migrate.WithModels(models.User{}, models.Product{}),
		migrate.WithOutputDir("./migrations"),
		migrate.WithDriver("postgres"),
	)
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}

	intent := os.Args[1]
	res, err := orchestrator.Run(context.Background(), intent, orchestrator.Options{
		Migrator:    m,
		OutputDir:   "./migrations",
		DialectName: "postgres",
	})
	if err != nil {
		log.Fatalf("%s: %v", intent, err)
	}
}
```

### 3. Generate migrations

```bash
# Set database connection
export DATABASE_URL="postgres://user:pass@localhost/dbname"

# Generate migration
go run ./cmd/gms generate
```

### 4. Apply migrations

```bash
# Check status
gms status

# Apply pending migrations
gms apply

# Rollback last migration
gms rollback
```

## CLI Commands

| Command      | Description                                      |
|--------------|--------------------------------------------------|
| `generate`   | Generate migration files from struct models      |
| `apply`      | Apply pending migrations                         |
| `status`     | Show current migration status                    |
| `history`    | Show migration history                           |
| `validate`   | Validate all migration files                     |
| `rollback`   | Rollback last N migrations                       |
| `diff`       | Show what would change (no files written)        |
| `studio`     | Launch the GMS Studio web UI to browse/edit DB   |

## GMS Studio

GMS Studio is a built-in web-based database explorer (inspired by Prisma Studio) that allows you to browse tables, view data, and perform CRUD operations directly from your browser.

To launch it:
```bash
# It will auto-detect your DATABASE_URL
gms studio

# Or specify a driver and address
gms studio --driver postgres --addr 127.0.0.1:4489
```

Features:
- **Modern Dark UI**: Clean and responsive interface.
- **CRUD Operations**: Add, edit, and delete records with a custom confirmation modal.
- **Schema Awareness**: Automatically detects primary keys and data types for safe editing.
- **Smart Paging**: Efficiently handle large tables with pagination and search.

## Supported Databases

- PostgreSQL
- MySQL
- SQLite
- SQL Server
- MongoDB

## Struct Tags

| Tag Option       | Description                                      |
|------------------|--------------------------------------------------|
| `table:name`     | Override table name (use on dummy `_ struct{}`)  |
| `table_old_name` | Old table name for renaming (use on dummy struct)|
| `pk`             | Mark as primary key                              |
| `autoincrement`  | Auto-incrementing primary key                    |
| `unique`         | Unique constraint                                |
| `index`          | Create index                                     |
| `not null`       | Not null constraint                              |
| `nullable`       | Nullable field                                   |
| `default:X`      | Default value                                    |
| `size:N`         | Column size (for VARCHAR, etc.)                  |
| `type:T`         | Override SQL type                                |
| `old_name:X`     | Old column name for renaming                     |
| `check:expr`     | Check constraint (e.g. `check:price >= 0`)       |
| `ignore`         | Ignore this field                                |

## Configuration

### Environment Variables

| Variable         | Description                                      |
|------------------|--------------------------------------------------|
| `DATABASE_URL`   | Database connection string                       |
| `MYSQL_DSN`      | MySQL DSN override                               |
| `PGSQL_DSN`      | PostgreSQL DSN override                          |
| `SQLITE_PATH`    | SQLite file path override                        |
| `MSSQL_DSN`      | SQL Server DSN override                          |
| `MONGODB_URI`    | MongoDB URI override                              |

### Safety Options

By default, destructive operations are blocked:
```go
m, err := migrate.New(
    migrate.WithModels(models...),
    migrate.WithAllowDropTable(),     // Allow DROP TABLE
    migrate.WithAllowDropColumn(),    // Allow DROP COLUMN
    migrate.WithAllowTypeChange(migrate.TypeChangeSafe), // Allow safe type changes
)
```

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines.

## License

MIT License - see [LICENSE](./LICENSE) file for details.
