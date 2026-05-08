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

func ListMyOrders(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")

		cur, err := db.Collection("orders").Find(c.Request.Context(),
			bson.M{"user_id": uid},
			options.Find().SetSort(bson.M{"created_at": -1}),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer cur.Close(c.Request.Context())

		type order struct {
			ID        string  `json:"id"`
			Status    string  `json:"status"`
			Total     float64 `json:"total"`
			CreatedAt string  `json:"created_at"`
		}
		var out []order
		for cur.Next(c.Request.Context()) {
			var raw struct {
				ID        primitive.ObjectID `bson:"_id"`
				Status    string             `bson:"status"`
				Total     float64            `bson:"total"`
				CreatedAt primitive.DateTime  `bson:"created_at"`
			}
			if err := cur.Decode(&raw); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			out = append(out, order{
				ID:        raw.ID.Hex(),
				Status:    raw.Status,
				Total:     raw.Total,
				CreatedAt: raw.CreatedAt.Time().Format("2006-01-02T15:04:05Z"),
			})
		}
		c.JSON(http.StatusOK, out)
	}
}

func GetOrder(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		role, _ := c.Get("role")

		oid, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var raw struct {
			ID        primitive.ObjectID `bson:"_id"`
			UserID    string             `bson:"user_id"`
			Status    string             `bson:"status"`
			Total     float64            `bson:"total"`
			Items     []struct {
				ProductID string  `bson:"product_id"`
				Name      string  `bson:"name"`
				Quantity  int     `bson:"quantity"`
				Price     float64 `bson:"price"`
			} `bson:"items"`
			CreatedAt primitive.DateTime `bson:"created_at"`
		}
		err = db.Collection("orders").FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&raw)
		if err == mongo.ErrNoDocuments {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if raw.UserID != uid.(string) && role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			return
		}

		type item struct {
			ProductID   string  `json:"product_id"`
			ProductName string  `json:"product_name"`
			Quantity    int     `json:"quantity"`
			Price       float64 `json:"price"`
		}
		items := make([]item, len(raw.Items))
		for i, it := range raw.Items {
			items[i] = item{
				ProductID:   it.ProductID,
				ProductName: it.Name,
				Quantity:    it.Quantity,
				Price:       it.Price,
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"id":         raw.ID.Hex(),
			"status":     raw.Status,
			"total":      raw.Total,
			"created_at": raw.CreatedAt.Time().Format("2006-01-02T15:04:05Z"),
			"items":      items,
		})
	}
}

type createOrderInput struct {
	Items []createOrderItem `json:"items" binding:"required,min=1"`
}

type createOrderItem struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

func CreateOrder(db *mongo.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, _ := c.Get("user_id")

		var in createOrderInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		type itemInfo struct {
			Price float64
			Stock int
			Name  string
		}
		items := make([]itemInfo, len(in.Items))

		for i, it := range in.Items {
			oid, err := primitive.ObjectIDFromHex(it.ProductID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product_id"})
				return
			}

			var prod struct {
				Price float64 `bson:"price"`
				Stock int     `bson:"stock"`
				Name  string  `bson:"name"`
			}
			err = db.Collection("products").FindOne(c.Request.Context(), bson.M{"_id": oid}).Decode(&prod)
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusBadRequest, gin.H{"error": "product not found", "product_id": it.ProductID})
				return
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if prod.Stock < it.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{"error": "insufficient stock", "product_id": it.ProductID})
				return
			}
			items[i] = itemInfo{Price: prod.Price, Stock: prod.Stock, Name: prod.Name}
		}

		var total float64
		orderItems := make([]struct {
			ProductID string  `bson:"product_id"`
			Name      string  `bson:"name"`
			Quantity  int     `bson:"quantity"`
			Price     float64 `bson:"price"`
		}, len(in.Items))

		for i, it := range in.Items {
			total += items[i].Price * float64(it.Quantity)
			orderItems[i] = struct {
				ProductID string  `bson:"product_id"`
				Name      string  `bson:"name"`
				Quantity  int     `bson:"quantity"`
				Price     float64 `bson:"price"`
			}{
				ProductID: it.ProductID,
				Name:      items[i].Name,
				Quantity:  it.Quantity,
				Price:     items[i].Price,
			}
		}

		now := time.Now().UTC()
		ord := struct {
			UserID    string            `bson:"user_id"`
			Status    string            `bson:"status"`
			Total     float64           `bson:"total"`
			Items     any               `bson:"items"`
			CreatedAt primitive.DateTime `bson:"created_at"`
			UpdatedAt primitive.DateTime `bson:"updated_at"`
		}{
			UserID:    uid.(string),
			Status:    "pending",
			Total:     total,
			Items:     orderItems,
			CreatedAt: primitive.NewDateTimeFromTime(now),
			UpdatedAt: primitive.NewDateTimeFromTime(now),
		}

		res, err := db.Collection("orders").InsertOne(c.Request.Context(), ord)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		for _, it := range in.Items {
			oid, _ := primitive.ObjectIDFromHex(it.ProductID)
			_, err := db.Collection("products").UpdateOne(c.Request.Context(),
				bson.M{"_id": oid},
				bson.M{"$inc": bson.M{"stock": -it.Quantity}},
			)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}

		orderID := res.InsertedID.(primitive.ObjectID).Hex()
		c.JSON(http.StatusCreated, gin.H{
			"id":    orderID,
			"total": total,
			"items": len(in.Items),
		})
	}
}
