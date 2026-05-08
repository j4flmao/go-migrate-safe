package internal

import "time"

type User struct {
	ID           int64     `db:"id,pk,autoincrement"`
	Username     string    `db:"username,unique,not null"`
	Email        string    `db:"email,unique,not null"`
	PasswordHash string    `db:"password_hash,not null"`
	Role         string    `db:"role,not null,default:user"`
	CreatedAt    time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `db:"updated_at,not null,default:CURRENT_TIMESTAMP"`
}

type Category struct {
	ID          int64     `db:"id,pk,autoincrement"`
	Name        string    `db:"name,not null"`
	Slug        string    `db:"slug,unique,not null"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
}

type Product struct {
	ID          int64     `db:"id,pk,autoincrement"`
	CategoryID  int64     `db:"category_id,not null,index,fk:categories(id)"`
	Name        string    `db:"name,not null"`
	Slug        string    `db:"slug,unique,not null"`
	Description string    `db:"description"`
	Price       float64   `db:"price,not null,default:0.00"`
	Stock       int       `db:"stock,not null,default:0"`
	ImageURL    string    `db:"image_url"`
	CreatedAt   time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time `db:"updated_at,not null,default:CURRENT_TIMESTAMP"`
}

type Order struct {
	ID        int64     `db:"id,pk,autoincrement"`
	UserID    int64     `db:"user_id,not null,index,fk:users(id)"`
	Status    string    `db:"status,not null,default:pending"`
	Total     float64   `db:"total,not null,default:0.00"`
	CreatedAt time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `db:"updated_at,not null,default:CURRENT_TIMESTAMP"`
}

type OrderItem struct {
	ID        int64     `db:"id,pk,autoincrement"`
	OrderID   int64     `db:"order_id,not null,fk:orders(id)"`
	ProductID int64     `db:"product_id,not null,fk:products(id)"`
	Quantity  int       `db:"quantity,not null,default:1"`
	Price     float64   `db:"price,not null"`
	CreatedAt time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
}

func Models() []any {
	return []any{User{}, Category{}, Product{}, Order{}, OrderItem{}}
}
