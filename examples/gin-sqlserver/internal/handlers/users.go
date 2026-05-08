package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/j4flmao/go-migrate-safe/examples/gin-sqlserver/internal"
)

func GetMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		c.JSON(http.StatusOK, gin.H{
			"user_id":  uid,
			"username": username,
			"role":     role,
		})
	}
}

func GetProfile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var user internal.User
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT id, username, email, role, created_at, updated_at FROM users WHERE id = @p1", id,
		).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		uid, _ := c.Get("user_id")
		if uid.(int64) != user.ID && c.GetString("role") != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		})
	}
}
