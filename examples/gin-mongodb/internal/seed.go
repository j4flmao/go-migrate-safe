package internal

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func SeedRun(uri string) {
	ctx := context.Background()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatalf("mongodb: %v", err)
	}
	defer client.Disconnect(ctx)

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongodb: ping: %v", err)
	}

	db := client.Database("go_migrate_example")

	n, err := db.Collection("users").CountDocuments(ctx, bson.M{})
	if err != nil {
		log.Fatalf("seed check: %v", err)
	}
	if n > 0 {
		log.Print("Database already has data, skipping seed")
		return
	}

	now := time.Now().UTC()

	adminHash, _ := HashPassword("admin123")
	admin := User{
		Username:     "admin",
		Email:        "admin@example.com",
		PasswordHash: adminHash,
		Role:         "admin",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	ar, err := db.Collection("users").InsertOne(ctx, admin)
	if err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	adminID := ar.InsertedID.(primitive.ObjectID).Hex()

	userHash, _ := HashPassword("user123")
	user := User{
		Username:     "john",
		Email:        "john@example.com",
		PasswordHash: userHash,
		Role:         "user",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err = db.Collection("users").InsertOne(ctx, user)
	if err != nil {
		log.Fatalf("seed user: %v", err)
	}

	categories := []struct{
		Name, Slug, Description string
	}{
		{"Electronics", "electronics", "Gadgets, devices, and tech accessories"},
		{"Clothing", "clothing", "Apparel and fashion items"},
		{"Books", "books", "Physical and digital books"},
	}

	catIDs := make(map[string]string)
	for _, c := range categories {
		cr, err := db.Collection("categories").InsertOne(ctx, ProductCategory{
			Name:        c.Name,
			Slug:        c.Slug,
			Description: c.Description,
			CreatedAt:   now,
		})
		if err != nil {
			log.Fatalf("seed category %s: %v", c.Slug, err)
		}
		catIDs[c.Slug] = cr.InsertedID.(primitive.ObjectID).Hex()
	}

	products := []struct {
		CategorySlug       string
		Name, Slug, Details string
		Price              float64
		Stock              int
		ImageURL           string
	}{
		{"electronics", "Wireless Headphones", "wireless-headphones", "Noise cancelling wireless headphones", 149.99, 50, ""},
		{"electronics", "USB-C Hub", "usb-c-hub", "7-in-1 USB-C hub with HDMI", 39.99, 120, ""},
		{"electronics", "Mechanical Keyboard", "mechanical-keyboard", "RGB mechanical keyboard, blue switches", 89.99, 30, ""},
		{"clothing", "Cotton T-Shirt", "cotton-tshirt", "Comfortable cotton t-shirt", 24.99, 200, ""},
		{"clothing", "Denim Jacket", "denim-jacket", "Classic blue denim jacket", 79.99, 15, ""},
		{"books", "Go Programming", "go-programming", "A comprehensive guide to Go", 44.99, 100, ""},
		{"books", "Clean Code", "clean-code", "Best practices for writing maintainable code", 34.99, 75, ""},
	}

	catNameLookup := map[string]string{
		"electronics": "Electronics",
		"clothing":    "Clothing",
		"books":       "Books",
	}

	for _, p := range products {
		catID := catIDs[p.CategorySlug]
		catName := catNameLookup[p.CategorySlug]
		_, err := db.Collection("products").InsertOne(ctx, Product{
			CategoryID:   catID,
			CategoryName: catName,
			Name:         p.Name,
			Slug:         p.Slug,
			Details:      p.Details,
			Price:        p.Price,
			Stock:        p.Stock,
			ImageURL:     p.ImageURL,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
		if err != nil {
			log.Fatalf("seed product %s: %v", p.Slug, err)
		}
	}

	log.Printf("Seed completed: admin (admin@example.com / admin123, id=%s), user (john@example.com / user123), 3 categories, 7 products", adminID)
}
