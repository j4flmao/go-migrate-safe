package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func ListCategories(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := db.QueryContext(c.Request.Context(),
			"SELECT id, name, slug, description, created_at FROM categories ORDER BY name")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type cat struct {
			ID          int64     `json:"id"`
			Name        string    `json:"name"`
			Slug        string    `json:"slug"`
			Description string    `json:"description"`
			CreatedAt   time.Time `json:"created_at"`
		}
		out := []cat{}
		for rows.Next() {
			var x cat
			if err := rows.Scan(&x.ID, &x.Name, &x.Slug, &x.Description, &x.CreatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = append(out, x)
		}
		c.JSON(http.StatusOK, out)
	}
}

func GetCategory(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var out struct {
			ID          int64     `json:"id"`
			Name        string    `json:"name"`
			Slug        string    `json:"slug"`
			Description string    `json:"description"`
			CreatedAt   time.Time `json:"created_at"`
		}
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT id, name, slug, description, created_at FROM categories WHERE id = $1", c.Param("id"),
		).Scan(&out.ID, &out.Name, &out.Slug, &out.Description, &out.CreatedAt)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func CreateCategory(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			Name        string `json:"name" binding:"required"`
			Slug        string `json:"slug" binding:"required"`
			Description string `json:"description"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var id int64
		err := db.QueryRowContext(c.Request.Context(),
			"INSERT INTO categories (name, slug, description) VALUES ($1, $2, $3) RETURNING id",
			in.Name, in.Slug, in.Description).Scan(&id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"id": id, "name": in.Name, "slug": in.Slug})
	}
}
