package internal

import (
	"context"
	"database/sql"
	"log"
)

func SeedRun(dsn string) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("db: %v", err)
	}

	ctx := context.Background()

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		log.Fatalf("seed check: %v", err)
	}
	if count > 0 {
		log.Print("Database already has data, skipping seed")
		return
	}

	adminHash, _ := HashPassword("admin123")
	_, err = db.ExecContext(ctx,
		"INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)",
		"admin", "admin@example.com", adminHash, "admin")
	if err != nil {
		log.Fatalf("seed admin: %v", err)
	}

	userHash, _ := HashPassword("user123")
	_, err = db.ExecContext(ctx,
		"INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)",
		"john", "john@example.com", userHash, "user")
	if err != nil {
		log.Fatalf("seed user: %v", err)
	}

	categories := []struct{ Name, Slug, Desc string }{
		{"Electronics", "electronics", "Gadgets, devices, and tech accessories"},
		{"Clothing", "clothing", "Apparel and fashion items"},
		{"Books", "books", "Physical and digital books"},
	}
	for _, c := range categories {
		_, err = db.ExecContext(ctx,
			"INSERT INTO categories (name, slug, description) VALUES (?, ?, ?)",
			c.Name, c.Slug, c.Desc)
		if err != nil {
			log.Fatalf("seed category %s: %v", c.Slug, err)
		}
	}

	_, err = db.ExecContext(ctx, `INSERT INTO products (category_id, name, slug, description, price, stock, image_url) VALUES
		(1, 'Wireless Headphones', 'wireless-headphones', 'Noise-cancelling Bluetooth headphones', 149.99, 50, ''),
		(1, 'USB-C Hub', 'usb-c-hub', '7-in-1 USB-C hub with HDMI', 39.99, 120, ''),
		(1, 'Mechanical Keyboard', 'mechanical-keyboard', 'RGB mechanical keyboard, blue switches', 89.99, 30, ''),
		(2, 'Cotton T-Shirt', 'cotton-tshirt', 'Soft 100% cotton t-shirt', 24.99, 200, ''),
		(2, 'Denim Jacket', 'denim-jacket', 'Classic blue denim jacket', 79.99, 15, ''),
		(3, 'Go Programming', 'go-programming', 'Comprehensive guide to Go', 44.99, 100, ''),
		(3, 'Clean Code', 'clean-code', 'Best practices for writing maintainable code', 34.99, 75, '')`)
	if err != nil {
		log.Fatalf("seed products: %v", err)
	}

	log.Print("Seed completed: admin (admin@example.com / admin123), user (john@example.com / user123), 3 categories, 7 products")
}
