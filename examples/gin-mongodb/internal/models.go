package internal

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID           primitive.ObjectID `db:"id,pk" bson:"_id,omitempty" json:"id"`
	Username     string             `db:"username,unique,not null" bson:"username" json:"username"`
	Email        string             `db:"email,unique,not null" bson:"email" json:"email"`
	PasswordHash string             `db:"password_hash,not null" bson:"password_hash" json:"-"`
	Role         string             `db:"role,not null,default:user" bson:"role" json:"role"`
	CreatedAt    time.Time          `db:"created_at,not null" bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `db:"updated_at,not null" bson:"updated_at" json:"updated_at"`
}

type Category struct {
	ID          primitive.ObjectID `db:"id,pk" bson:"_id,omitempty" json:"id"`
	Name        string             `db:"name,not null" bson:"name" json:"name"`
	Slug        string             `db:"slug,unique,not null" bson:"slug" json:"slug"`
	Description string             `db:"description" bson:"description" json:"description"`
	CreatedAt   time.Time          `db:"created_at,not null" bson:"created_at" json:"created_at"`
}

type Product struct {
	ID           primitive.ObjectID `db:"id,pk" bson:"_id,omitempty" json:"id"`
	CategoryID   string             `db:"category_id,not null,index" bson:"category_id" json:"category_id"`
	CategoryName string             `db:"category_name" bson:"category_name,omitempty" json:"category_name,omitempty"`
	Name         string             `db:"name,not null" bson:"name" json:"name"`
	Slug         string             `db:"slug,unique,not null" bson:"slug" json:"slug"`
	Description  string             `db:"description" bson:"description" json:"description"`
	Price        float64            `db:"price,not null" bson:"price" json:"price"`
	Stock        int                `db:"stock,not null" bson:"stock" json:"stock"`
	ImageURL     string             `db:"image_url" bson:"image_url" json:"image_url"`
	CreatedAt    time.Time          `db:"created_at,not null" bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `db:"updated_at,not null" bson:"updated_at" json:"updated_at"`
}

type Order struct {
	ID        primitive.ObjectID `db:"id,pk" bson:"_id,omitempty" json:"id"`
	UserID    string             `db:"user_id,not null,index" bson:"user_id" json:"user_id"`
	Status    string             `db:"status,not null,default:pending" bson:"status" json:"status"`
	Total     float64            `db:"total,not null" bson:"total" json:"total"`
	Items     []OrderItem        `db:"-" bson:"items" json:"items"`
	CreatedAt time.Time          `db:"created_at,not null" bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `db:"updated_at,not null" bson:"updated_at" json:"updated_at"`
}

type OrderItem struct {
	ProductID string  `bson:"product_id" json:"product_id"`
	Name      string  `bson:"name" json:"name"`
	Quantity  int     `bson:"quantity" json:"quantity"`
	Price     float64 `bson:"price" json:"price"`
}

func Models() []any {
	return []any{User{}, Category{}, Product{}, Order{}}
}
