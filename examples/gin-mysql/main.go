package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func dsn() string {
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		return v
	}
	return "root@tcp(127.0.0.1:3306)/go_migrate_example?parseTime=true&charset=utf8mb4"
}

func main() {
	db, err := sql.Open("mysql", dsn())
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v (is MySQL running?)", err)
	}

	r := newRouter(db)
	addr := ":8081"
	if v := os.Getenv("LISTEN"); v != "" {
		addr = v
	}
	log.Printf("Listening on %s", addr)
	r.Run(addr)
}
