package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/j4flmao/go-migrate-safe/examples/gin-mongodb/internal"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func Register(db *mongo.Database) gin.HandlerFunc {
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

		user := internal.User{
			Username:     in.Username,
			Email:        in.Email,
			PasswordHash: hash,
			Role:         "user",
		}
		res, err := db.Collection("users").InsertOne(c.Request.Context(), user)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		oid := res.InsertedID.(interface{ Hex() string }).Hex()
		token, _ := internal.GenerateToken(oid, in.Username, "user")
		c.JSON(http.StatusCreated, AuthResponse{Token: token, UserID: oid, Username: in.Username, Role: "user"})
	}
}

func Login(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in LoginInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var u internal.User
		err := db.Collection("users").FindOne(c.Request.Context(), bson.M{"email": in.Email}).Decode(&u)
		if err == mongo.ErrNoDocuments {
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

		token, _ := internal.GenerateToken(u.ID.Hex(), u.Username, u.Role)
		c.JSON(http.StatusOK, AuthResponse{Token: token, UserID: u.ID.Hex(), Username: u.Username, Role: u.Role})
	}
}
