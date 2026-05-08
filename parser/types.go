package parser

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Dialect identifies the SQL dialect for type mapping.
type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	DialectMySQL    Dialect = "mysql"
	DialectSQLite   Dialect = "sqlite"
	DialectMongoDB  Dialect = "mongodb"
	DialectMSSQL    Dialect = "mssql"
)

// goSQLType maps a Go reflect.Type to its canonical SQL type for the dialect.
// Returns the type string and a "isNullable hint" derived from pointer status.
//
// Per data-model.md §6.1.
func goSQLType(t reflect.Type, d Dialect) (string, error) {
	// Unwrap pointers (nullable markers).
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Special types.
	switch t {
	case reflect.TypeOf(time.Time{}):
		switch d {
		case DialectPostgres:
			return "TIMESTAMPTZ", nil
		case DialectMySQL:
			return "DATETIME", nil
		case DialectSQLite:
			return "TEXT", nil
		case DialectMongoDB:
			return "DATETIME", nil
		case DialectMSSQL:
			return "DATETIME2", nil
		}
	}

	// Common stdlib types matched by name to avoid importing every package.
	pkgPath := t.PkgPath()
	name := t.Name()
	if pkgPath == "database/sql" {
		switch name {
		case "NullString":
			return mapKind(reflect.String, d), nil
		case "NullInt64":
			return mapKind(reflect.Int64, d), nil
		case "NullInt32":
			return mapKind(reflect.Int32, d), nil
		case "NullBool":
			return mapKind(reflect.Bool, d), nil
		case "NullFloat64":
			return mapKind(reflect.Float64, d), nil
		case "NullTime":
			switch d {
			case DialectPostgres:
				return "TIMESTAMPTZ", nil
			case DialectMySQL:
				return "DATETIME", nil
			case DialectSQLite:
				return "TEXT", nil
			case DialectMongoDB:
				return "DATETIME", nil
			case DialectMSSQL:
				return "DATETIME2", nil
			}
		}
	}

	// Handle primitive.ObjectID (MongoDB driver).
	if pkgPath == "go.mongodb.org/mongo-driver/bson/primitive" && name == "ObjectID" {
		return "TEXT", nil
	}

	// []byte
	if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
		switch d {
		case DialectPostgres:
			return "BYTEA", nil
		case DialectMySQL:
			return "BLOB", nil
		case DialectSQLite:
			return "BLOB", nil
		case DialectMSSQL:
			return "VARBINARY(MAX)", nil
		}
	}

	if s := mapKind(t.Kind(), d); s != "" {
		return s, nil
	}
	return "", fmt.Errorf("unsupported Go type for SQL mapping: %s (kind=%s)", t.String(), t.Kind())
}

func mapKind(k reflect.Kind, d Dialect) string {
	switch k {
	case reflect.Bool:
		switch d {
		case DialectPostgres:
			return "BOOLEAN"
		case DialectMySQL:
			return "TINYINT(1)"
		case DialectSQLite:
			return "INTEGER"
		case DialectMongoDB:
			return "BOOLEAN"
		case DialectMSSQL:
			return "BIT"
		}
	case reflect.Int8, reflect.Int16:
		switch d {
		case DialectPostgres, DialectMySQL:
			return "SMALLINT"
		case DialectSQLite:
			return "INTEGER"
		case DialectMongoDB:
			return "INTEGER"
		case DialectMSSQL:
			return "SMALLINT"
		}
	case reflect.Int, reflect.Int32:
		switch d {
		case DialectPostgres:
			return "INTEGER"
		case DialectMySQL:
			return "INT"
		case DialectSQLite:
			return "INTEGER"
		case DialectMongoDB:
			return "INTEGER"
		case DialectMSSQL:
			return "INT"
		}
	case reflect.Int64:
		switch d {
		case DialectPostgres, DialectMySQL:
			return "BIGINT"
		case DialectSQLite:
			return "INTEGER"
		case DialectMongoDB:
			return "INTEGER"
		case DialectMSSQL:
			return "BIGINT"
		}
	case reflect.Uint, reflect.Uint32:
		switch d {
		case DialectPostgres:
			return "BIGINT"
		case DialectMySQL:
			return "INT UNSIGNED"
		case DialectSQLite:
			return "INTEGER"
		case DialectMongoDB:
			return "INTEGER"
		case DialectMSSQL:
			return "BIGINT"
		}
	case reflect.Uint64:
		switch d {
		case DialectPostgres:
			return "NUMERIC(20,0)"
		case DialectMySQL:
			return "BIGINT UNSIGNED"
		case DialectSQLite:
			return "INTEGER"
		case DialectMongoDB:
			return "DOUBLE"
		case DialectMSSQL:
			return "NUMERIC(20,0)"
		}
	case reflect.Float32:
		switch d {
		case DialectPostgres:
			return "REAL"
		case DialectMySQL:
			return "FLOAT"
		case DialectSQLite:
			return "REAL"
		case DialectMongoDB:
			return "DOUBLE"
		case DialectMSSQL:
			return "REAL"
		}
	case reflect.Float64:
		switch d {
		case DialectPostgres:
			return "DOUBLE PRECISION"
		case DialectMySQL:
			return "DOUBLE"
		case DialectSQLite:
			return "REAL"
		case DialectMongoDB:
			return "DOUBLE"
		case DialectMSSQL:
			return "FLOAT"
		}
	case reflect.String:
		switch d {
		case DialectPostgres, DialectMySQL, DialectSQLite:
			return "TEXT"
		case DialectMongoDB:
			return "TEXT"
		case DialectMSSQL:
			return "NVARCHAR(MAX)"
		}
	}
	return ""
}

// NormalizeType returns a canonical, comparable form of a SQL type string.
// Used by both the parser and the DB reader so the diff engine can compare.
func NormalizeType(s string) string {
	u := strings.ToUpper(strings.TrimSpace(s))
	// Collapse whitespace.
	u = strings.Join(strings.Fields(u), " ")
	switch u {
	case "CHARACTER VARYING", "VARCHAR":
		return "TEXT"
	case "INTEGER", "INT4", "INT":
		return "INTEGER"
	case "BIGINT", "INT8":
		return "BIGINT"
	case "SMALLINT", "INT2":
		return "SMALLINT"
	case "BOOLEAN", "BOOL", "TINYINT(1)":
		return "BOOLEAN"
	case "TIMESTAMP WITH TIME ZONE", "TIMESTAMPTZ":
		return "TIMESTAMPTZ"
	case "TIMESTAMP WITHOUT TIME ZONE", "TIMESTAMP":
		return "TIMESTAMP"
	case "DOUBLE PRECISION", "DOUBLE", "FLOAT8":
		return "DOUBLE"
	case "REAL", "FLOAT4", "FLOAT":
		return "FLOAT"
	case "BYTEA", "BLOB", "BINARY":
		return "BYTES"
	case "JSON":
		return "JSON"
	case "JSONB":
		return "JSONB"
	case "UUID":
		return "UUID"
	case "TEXT", "LONGTEXT", "MEDIUMTEXT":
		return "TEXT"
	}
	// Preserve parameterized types like VARCHAR(50), DECIMAL(12,4).
	return u
}
