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

type productResp struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	Stock        int     `json:"stock"`
	ImageURL     string  `json:"image_url"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	CreatedAt    string  `json:"created_at"`
}

func toProductResp(raw struct {
	ID           primitive.ObjectID `bson:"_id"`
	CategoryID   string             `bson:"category_id"`
	CategoryName string             `bson:"category_name,omitempty"`
	Name         string             `bson:"name"`
	Slug         string             `bson:"slug"`
	Description  string             `bson:"description"`
	Price        float64            `bson:"price"`
	Stock        int                `bson:"stock"`
	ImageURL     string             `bson:"image_url"`
	CreatedAt    primitive.DateTime `bson:"created_at"`
}) productResp {
	return productResp{
		ID:           raw.ID.Hex(),
		Name:         raw.Name,
		Slug:         raw.Slug,
		Description:  raw.Description,
		Price:        raw.Price,
		Stock:        raw.Stock,
		ImageURL:     raw.ImageURL,
		CategoryID:   raw.CategoryID,
		CategoryName: raw.CategoryName,
		CreatedAt:    raw.CreatedAt.Time().Format("2006-01-02T15:04:05Z"),
	}
}

func ListProducts(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		filter := bson.M{}
		if catID := c.Query("category_id"); catID != "" {
			filter["category_id"] = catID
		}

		cur, err := db.Collection("products").Find(c.Request.Context(), filter, options.Find().SetSort(bson.M{"name": 1}))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cur.Close(c.Request.Context())

		var out []productResp
		for cur.Next(c.Request.Context()) {
			var raw struct {
				ID           primitive.ObjectID `bson:"_id"`
				CategoryID   string             `bson:"category_id"`
				CategoryName string             `bson:"category_name,omitempty"`
				Name         string             `bson:"name"`
				Slug         string             `bson:"slug"`
				Description  string             `bson:"description"`
				Price        float64            `bson:"price"`
				Stock        int                `bson:"stock"`
				ImageURL     string             `bson:"image_url"`
				CreatedAt    primitive.DateTime  `bson:"created_at"`
			}
			if err := cur.Decode(&raw); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = append(out, toProductResp(raw))
		}
		c.JSON(http.StatusOK, out)
	}
}

func GetProduct(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		oid, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var raw struct {
			ID           primitive.ObjectID `bson:"_id"`
			CategoryID   string             `bson:"category_id"`
			CategoryName string             `bson:"category_name,omitempty"`
			Name         string             `bson:"name"`
			Slug         string             `bson:"slug"`
			Description  string             `bson:"description"`
			Price        float64            `bson:"price"`
			Stock        int                `bson:"stock"`
			ImageURL     string             `bson:"image_url"`
			CreatedAt    primitive.DateTime  `bson:"created_at"`
		}
		err = db.Collection("products").FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&raw)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, toProductResp(raw))
	}
}

func CreateProduct(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in struct {
			CategoryID  string  `json:"category_id" binding:"required"`
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

		catOID, err := primitive.ObjectIDFromHex(in.CategoryID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
			return
		}

		var cat struct {
			Name string `bson:"name"`
		}
		if err := db.Collection("categories").FindOne(c.Request.Context(), bson.M{"_id": catOID}).Decode(&cat); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
			return
		}

		now := time.Now().UTC()
		doc := struct {
			CategoryID   string            `bson:"category_id"`
			CategoryName string            `bson:"category_name"`
			Name         string            `bson:"name"`
			Slug         string            `bson:"slug"`
			Description  string            `bson:"description"`
			Price        float64           `bson:"price"`
			Stock        int               `bson:"stock"`
			ImageURL     string            `bson:"image_url"`
			CreatedAt    primitive.DateTime `bson:"created_at"`
			UpdatedAt    primitive.DateTime `bson:"updated_at"`
		}{
			CategoryID:   in.CategoryID,
			CategoryName: cat.Name,
			Name:         in.Name,
			Slug:         in.Slug,
			Description:  in.Description,
			Price:        in.Price,
			Stock:        in.Stock,
			ImageURL:     in.ImageURL,
			CreatedAt:    primitive.NewDateTimeFromTime(now),
			UpdatedAt:    primitive.NewDateTimeFromTime(now),
		}
		res, err := db.Collection("products").InsertOne(c.Request.Context(), doc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		id := res.InsertedID.(primitive.ObjectID).Hex()
		c.JSON(http.StatusCreated, gin.H{"id": id, "name": in.Name, "slug": in.Slug, "price": in.Price})
	}
}
