package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/j4flmao/go-migrate-safe/examples/gin-sqlserver/internal"
)

type RegisterInput struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginInput struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token    string `json:"token"`
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func Register(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in RegisterInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		hash, err := internal.HashPassword(in.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
			return
		}

		var uid int64
		err = db.QueryRowContext(c.Request.Context(),
			"INSERT INTO users (username, email, password_hash) VALUES (@p1, @p2, @p3); SELECT SCOPE_IDENTITY()",
			in.Username, in.Email, hash).Scan(&uid)
		if err != nil {
			if isDuplicate(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		token, _ := internal.GenerateToken(uid, in.Username, "user")
		c.JSON(http.StatusCreated, AuthResponse{Token: token, UserID: uid, Username: in.Username, Role: "user"})
	}
}

func Login(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in LoginInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var u struct {
			ID           int64
			Username     string
			PasswordHash string
			Role         string
		}
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT id, username, password_hash, role FROM users WHERE email = @p1", in.Email,
		).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if !internal.CheckPassword(u.PasswordHash, in.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}

		token, _ := internal.GenerateToken(u.ID, u.Username, u.Role)
		c.JSON(http.StatusOK, AuthResponse{Token: token, UserID: u.ID, Username: u.Username, Role: u.Role})
	}
}

func isDuplicate(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "Violation of UNIQUE KEY constraint") || strings.Contains(err.Error(), "2601") || strings.Contains(err.Error(), "2627"))
}
