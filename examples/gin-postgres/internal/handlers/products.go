package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func ListProducts(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		catID := c.Query("category_id")

		query := `SELECT p.id, p.name, p.slug, p.description, p.price, p.stock, p.image_url,
			c.id, c.name, p.created_at
			FROM products p
			LEFT JOIN categories c ON c.id = p.category_id`
		args := []any{}

		if catID != "" {
			query += " WHERE p.category_id = $1"
			args = append(args, catID)
		}
		query += " ORDER BY p.name"

		rows, err := db.QueryContext(c.Request.Context(), query, args...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type prod struct {
			ID           int64     `json:"id"`
			Name         string    `json:"name"`
			Slug         string    `json:"slug"`
			Description  string    `json:"description"`
			Price        float64   `json:"price"`
			Stock        int       `json:"stock"`
			ImageURL     string    `json:"image_url"`
			CategoryID   *int64    `json:"category_id"`
			CategoryName *string   `json:"category_name"`
			CreatedAt    time.Time `json:"created_at"`
		}
		out := []prod{}
		for rows.Next() {
			var p prod
			var catID sql.NullInt64
			var catName sql.NullString
			if err := rows.Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price, &p.Stock, &p.ImageURL,
				&catID, &catName, &p.CreatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if catID.Valid {
				p.CategoryID = &catID.Int64
			}
			if catName.Valid {
				p.CategoryName = &catName.String
			}
			out = append(out, p)
		}
		c.JSON(http.StatusOK, out)
	}
}

func GetProduct(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p struct {
			ID           int64     `json:"id"`
			Name         string    `json:"name"`
			Slug         string    `json:"slug"`
			Description  string    `json:"description"`
			Price        float64   `json:"price"`
			Stock        int       `json:"stock"`
			ImageURL     string    `json:"image_url"`
			CategoryID   *int64    `json:"category_id"`
			CategoryName *string   `json:"category_name"`
			CreatedAt    time.Time `json:"created_at"`
		}
		var catID sql.NullInt64
		var catName sql.NullString
		err := db.QueryRowContext(c.Request.Context(),
			`SELECT p.id, p.name, p.slug, p.description, p.price, p.stock, p.image_url,
				c.id, c.name, p.created_at
			 FROM products p
			 LEFT JOIN categories c ON c.id = p.category_id
			 WHERE p.id = $1`, c.Param("id"),
		).Scan(&p.ID, &p.Name, &p.Slug, &p.Description, &p.Price, &p.Stock, &p.ImageURL,
			&catID, &catName, &p.CreatedAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if catID.Valid {
			p.CategoryID = &catID.Int64
		}
		if catName.Valid {
			p.CategoryName = &catName.String
		}
		c.JSON(http.StatusOK, p)
	}
}

func CreateProduct(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			CategoryID  int64   `json:"category_id" binding:"required"`
			Name        string  `json:"name" binding:"required"`
			Slug        string  `json:"slug" binding:"required"`
			Description string  `json:"description"`
			Price       float64 `json:"price" binding:"required,gt=0"`
			Stock       int     `json:"stock"`
			ImageURL    string  `json:"image_url"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var id int64
		err := db.QueryRowContext(c.Request.Context(),
			`INSERT INTO products (category_id, name, slug, description, price, stock, image_url)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			in.CategoryID, in.Name, in.Slug, in.Description, in.Price, in.Stock, in.ImageURL).Scan(&id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id, "name": in.Name, "slug": in.Slug, "price": in.Price})
	}
}
