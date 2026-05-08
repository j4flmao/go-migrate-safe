package main

import (
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/gin-gonic/gin"
	"github.com/j4flmao/go-migrate-safe/examples/gin-mongodb/internal"
	"github.com/j4flmao/go-migrate-safe/examples/gin-mongodb/internal/handlers"
)

func newRouter(db *mongo.Database) *gin.Engine {
	r := gin.Default()

	api := r.Group("/api")
	{
		api.POST("/auth/register", handlers.Register(db))
		api.POST("/auth/login", handlers.Login(db))

		authed := api.Group("")
		authed.Use(internal.AuthRequired())
		{
			authed.GET("/me", handlers.GetMe())
			authed.GET("/users/:id", handlers.GetProfile(db))

			authed.GET("/categories", handlers.ListCategories(db))
			authed.GET("/categories/:id", handlers.GetCategory(db))

			authed.GET("/products", handlers.ListProducts(db))
			authed.GET("/products/:id", handlers.GetProduct(db))

			authed.GET("/orders", handlers.ListMyOrders(db))
			authed.GET("/orders/:id", handlers.GetOrder(db))
			authed.POST("/orders", handlers.CreateOrder(db))
		}

		admin := api.Group("")
		admin.Use(internal.AuthRequired())
		admin.Use(internal.AdminRequired())
		{
			admin.POST("/categories", handlers.CreateCategory(db))
			admin.POST("/products", handlers.CreateProduct(db))
		}
	}

	return r
}
