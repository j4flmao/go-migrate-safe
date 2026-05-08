package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ListMyOrders(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		rows, err := db.QueryContext(c.Request.Context(),
			"SELECT id, status, total, created_at FROM orders WHERE user_id = @p1 ORDER BY created_at DESC", uid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type order struct {
			ID        int64   `json:"id"`
			Status    string  `json:"status"`
			Total     float64 `json:"total"`
			CreatedAt string  `json:"created_at"`
		}
		var out []order
		for rows.Next() {
			var o order
			if err := rows.Scan(&o.ID, &o.Status, &o.Total, &o.CreatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = append(out, o)
		}
		c.JSON(http.StatusOK, out)
	}
}

func GetOrder(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		role, _ := c.Get("role")

		var o struct {
			ID        int64   `json:"id"`
			UserID    int64   `json:"user_id"`
			Status    string  `json:"status"`
			Total     float64 `json:"total"`
			CreatedAt string  `json:"created_at"`
		}
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT id, user_id, status, total, created_at FROM orders WHERE id = @p1", c.Param("id"),
		).Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.CreatedAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if o.UserID != uid.(int64) && role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		rows, err := db.QueryContext(c.Request.Context(),
			`SELECT oi.id, oi.quantity, oi.price, p.name, p.slug
			 FROM order_items oi
			 JOIN products p ON p.id = oi.product_id
			 WHERE oi.order_id = @p1`, o.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type item struct {
			ID           int64   `json:"id"`
			Quantity     int     `json:"quantity"`
			Price        float64 `json:"price"`
			ProductName  string  `json:"product_name"`
			ProductSlug  string  `json:"product_slug"`
		}
		var items []item
		for rows.Next() {
			var i item
			if err := rows.Scan(&i.ID, &i.Quantity, &i.Price, &i.ProductName, &i.ProductSlug); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			items = append(items, i)
		}

		c.JSON(http.StatusOK, gin.H{
			"id":         o.ID,
			"status":     o.Status,
			"total":      o.Total,
			"created_at": o.CreatedAt,
			"items":      items,
		})
	}
}

type createOrderInput struct {
	Items []createOrderItem `json:"items" binding:"required,min=1"`
}

type createOrderItem struct {
	ProductID int64 `json:"product_id" binding:"required"`
	Quantity  int   `json:"quantity" binding:"required,min=1"`
}

func CreateOrder(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")

		var in createOrderInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var total float64
		type itemInfo struct {
			Price float64
			Stock int
		}
		items := make([]itemInfo, len(in.Items))

		for i, it := range in.Items {
			var info itemInfo
			err := db.QueryRowContext(c.Request.Context(),
				"SELECT price, stock FROM products WHERE id = @p1", it.ProductID,
			).Scan(&info.Price, &info.Stock)
			if err == sql.ErrNoRows {
				c.JSON(http.StatusBadRequest, gin.H{"error": "product not found", "product_id": it.ProductID})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if info.Stock < it.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stock", "product_id": it.ProductID})
				return
			}
			items[i] = info
			total += info.Price * float64(it.Quantity)
		}

		tx, err := db.BeginTx(c.Request.Context(), nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer tx.Rollback()

		var orderID int64
		err = tx.QueryRowContext(c.Request.Context(),
			"INSERT INTO orders (user_id, total) VALUES (@p1, @p2); SELECT SCOPE_IDENTITY()", uid, total).Scan(&orderID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for i, it := range in.Items {
			_, err := tx.ExecContext(c.Request.Context(),
				"INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (@p1, @p2, @p3, @p4)",
				orderID, it.ProductID, it.Quantity, items[i].Price)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			_, err = tx.ExecContext(c.Request.Context(),
				"UPDATE products SET stock = stock - @p1 WHERE id = @p2", it.Quantity, it.ProductID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		if err := tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":    orderID,
			"total": total,
			"items": len(in.Items),
		})
	}
}
