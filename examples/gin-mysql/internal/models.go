package internal

import "time"

type User struct {
	ID           int64     `db:"id,pk,autoincrement"`
	Username     string    `db:"username,unique,not null,size:255"`
	Email        string    `db:"email,unique,not null,size:255"`
	PasswordHash string    `db:"password_hash,not null"`
	Role         string    `db:"role,not null,default:user"`
	CreatedAt    time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
	UpdatedAt    time.Time `db:"updated_at,not null,default:CURRENT_TIMESTAMP"`
}

type ProductCategory struct {
	_           struct{}  `db:"table:product_categories,table_old_name:categories"`
	ID          int64     `db:"id,pk,autoincrement"`
	Name        string    `db:"name,not null"`
	Slug        string    `db:"slug,unique,not null,size:255"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at,not null,default:CURRENT_TIMESTAMP"`
}

type Product struct {
	ID          int64     `db:"id,pk,autoincrement"`
	CategoryID  int64     `db:"category_id,not null,index,fk:product_categories(id)"`
	Name        string    `db:"name,not null"`
	Slug        string    `db:"slug,unique,not null,size:255"`
	Details     string    `db:"details,old_name:description"`
	Price       float64   `db:"price,not null,default:0.00,check:price >= 0"`
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
	return []any{User{}, ProductCategory{}, Product{}, Order{}, OrderItem{}}
}
