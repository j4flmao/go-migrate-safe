package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/j4flmao/go-migrate-safe/examples/gin-mongodb/internal"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

func GetProfile(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		oid, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var user internal.User
		err = db.Collection("users").FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&user)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		uid, _ := c.Get("user_id")
		if uid.(string) != user.ID.Hex() && c.GetString("role") != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":         user.ID.Hex(),
			"username":   user.Username,
			"email":      user.Email,
			"role":       user.Role,
			"created_at": user.CreatedAt,
		})
	}
}
