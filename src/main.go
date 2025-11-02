package main

import (
	"log"
	"os"
	"sync"
	
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

	// Determine database type from environment variable
	databaseType := os.Getenv("DATABASE_TYPE")
	if databaseType == "" {
		databaseType = "mysql" // Default to MySQL for backward compatibility
	}
	log.Printf("Using database type: %s", databaseType)

	// Initialize database based on type
	if databaseType == "dynamodb" {
		// Initialize DynamoDB
		log.Println("Initializing DynamoDB...")
		if err := InitDynamoDB(); err != nil {
			log.Fatalf("Failed to initialize DynamoDB: %v", err)
		}
		defer CloseDynamoDB()
		
		// Still initialize MySQL for product lookups (products table)
		log.Println("Initializing MySQL for product lookups...")
		if err := InitDatabase(); err != nil {
			log.Printf("Warning: Failed to initialize MySQL (products may not be available): %v", err)
		} else {
			defer CloseDatabase()
		}
	} else {
		// Initialize MySQL (default)
		log.Println("Initializing MySQL database...")
		if err := InitDatabase(); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer CloseDatabase()
	}

	// Generate and seed products (always needed for product lookups)
    log.Println("Generating products...")
    products := GenerateProducts(100000)
    
    // Seed products into MySQL database (if MySQL is available)
    if DB != nil {
		if err := seedProductsBatch(products); err != nil {
			log.Printf("Warning: Failed to seed products: %v", err)
		}
	}

	for k, v := range products {
		syncProducts.Store(k, v)
	}

	// initialize Gin router using Default
	router := gin.Default()

	// Health endpoint - checks appropriate database connection
    router.GET("/health", func(c *gin.Context) {
		if databaseType == "dynamodb" {
			// Check DynamoDB connection (describe table)
			if DynamoDBClient == nil {
				c.JSON(503, gin.H{
					"status": "unhealthy",
					"error":  "DynamoDB client not initialized",
				})
				return
			}
			c.JSON(200, gin.H{
				"status": "healthy",
				"database": "dynamodb",
			})
		} else {
			// Check MySQL connection
			if DB == nil {
				c.JSON(503, gin.H{
					"status": "unhealthy",
					"error":  "database connection not initialized",
				})
				return
			}
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
				"database": "mysql",
			})
		}
    })

	// Shopping cart endpoints - route to appropriate handlers based on database type
	if databaseType == "dynamodb" {
		router.POST("/shopping-carts", createShoppingCartDynamoDB)
		router.GET("/shopping-carts/:id", getShoppingCartDynamoDB)
		router.POST("/shopping-carts/:id/items", addItemToCartDynamoDB)
	} else {
		router.POST("/shopping-carts", createShoppingCart)
		router.GET("/shopping-carts/:id", getShoppingCart)
		router.POST("/shopping-carts/:id/items", addItemToCart)
	}
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
