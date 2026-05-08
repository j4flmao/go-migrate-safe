package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ListCategories(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		cur, err := db.Collection("categories").Find(c.Request.Context(), bson.M{}, options.Find().SetSort(bson.M{"name": 1}))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cur.Close(c.Request.Context())

		type cat struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Slug        string `json:"slug"`
			Description string `json:"description"`
			CreatedAt   string `json:"created_at"`
		}
		var out []cat
		for cur.Next(c.Request.Context()) {
			var raw struct {
				ID          primitive.ObjectID `bson:"_id"`
				Name        string             `bson:"name"`
				Slug        string             `bson:"slug"`
				Description string             `bson:"description"`
				CreatedAt   primitive.DateTime  `bson:"created_at"`
			}
			if err := cur.Decode(&raw); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = append(out, cat{
				ID:          raw.ID.Hex(),
				Name:        raw.Name,
				Slug:        raw.Slug,
				Description: raw.Description,
				CreatedAt:   raw.CreatedAt.Time().Format("2006-01-02T15:04:05Z"),
			})
		}
		c.JSON(http.StatusOK, out)
	}
}

func GetCategory(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		oid, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var raw struct {
			ID          primitive.ObjectID `bson:"_id"`
			Name        string             `bson:"name"`
			Slug        string             `bson:"slug"`
			Description string             `bson:"description"`
			CreatedAt   primitive.DateTime  `bson:"created_at"`
		}
		err = db.Collection("categories").FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&raw)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":          raw.ID.Hex(),
			"name":        raw.Name,
			"slug":        raw.Slug,
			"description": raw.Description,
			"created_at":  raw.CreatedAt.Time().Format("2006-01-02T15:04:05Z"),
		})
	}
}

func CreateCategory(db *mongo.Database) gin.HandlerFunc {
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

		cat := struct {
			Name        string             `bson:"name"`
			Slug        string             `bson:"slug"`
			Description string             `bson:"description"`
			CreatedAt   primitive.DateTime  `bson:"created_at"`
		}{
			Name:        in.Name,
			Slug:        in.Slug,
			Description: in.Description,
			CreatedAt:   primitive.NewDateTimeFromTime(time.Now().UTC()),
		}
		res, err := db.Collection("categories").InsertOne(c.Request.Context(), cat)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		id := res.InsertedID.(primitive.ObjectID).Hex()
		c.JSON(http.StatusCreated, gin.H{"id": id, "name": in.Name, "slug": in.Slug})
	}
}
