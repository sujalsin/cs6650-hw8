package main

import (
	"sync"
	"log"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// product map that stores all products
var syncProducts sync.Map
// var products map[int]Item

// Response structure
type SearchResponse struct {
	Products      []Item `json:"products"`
	TotalFound    int    `json:"total_found"`
	TotalSearched int    `json:"total_searched"`
	SearchTime    string `json:"search_time"`
}

// func init() {
// 	// Generate the products map
// 	products = GenerateProducts(100000)

	
// }

func main() {

	// Load .env file
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, using system environment variables")
    }

	// Initialize database connection and apply schema
    log.Println("Initializing database...")
    if err := InitDatabase(); err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer CloseDatabase()

	// Generate and seed products
    log.Println("Generating products...")
    products := GenerateProducts(100000)
    
    // Seed products into database
    if err := seedProductsBatch(products); err != nil {
        log.Fatalf("Failed to seed products: %v", err)
    }

	for k, v := range products {
		syncProducts.Store(k, v)
	}

	// initialize Gin router using Default
	router := gin.Default()

	// Health endpoint - now checks database connection
    router.GET("/health", func(c *gin.Context) {
        // Check if database connection is alive
        if err := DB.Ping(); err != nil {
            log.Printf("Health check failed: database connection error: %v", err)
            c.JSON(503, gin.H{
                "status": "unhealthy",
                "error":  "database connection failed",
            })
            return
        }
        
        c.JSON(200, gin.H{
            "status": "healthy",
            "database": "connected",
        })
    })

	// Shopping cart endpoints
    router.POST("/shopping-carts", createShoppingCart)
    router.GET("/shopping-carts/:id", getShoppingCart)
    router.POST("/shopping-carts/:id/items", addItemToCart)
	// associate GET HTTP method and "/products/{productId}" path with a handler function "getItemByID"
	router.GET("/products/:productId", getItemByID)
	// associate POST HTTP method and "/products/{productId}/details" path with a handler function "postItem"
	router.POST("/products/:productId/details", postItem)
	// associate GET HTTP method and "/products/search?q={query}" path with a handler function "searchProducts"
	router.GET("/products/search", searchProducts)
	printSample(products, 10)
	log.Printf("Total products: %d", len(products))
	// "Run()" attaches router to an http server and start the server
	router.Run(":8080")
}
