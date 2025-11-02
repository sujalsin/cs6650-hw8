package main

import (
    "database/sql"
    "log"
    "net/http"
    "strconv"
    "time"
	"math/rand"
    "fmt"
	"strings"
    "github.com/gin-gonic/gin"
)

// CartItem represents an item in the shopping cart
type CartItem struct {
    ID          int     `json:"id"`
    ProductID   int     `json:"product_id"`
    // ProductName string  `json:"product_name"`
    // SKU         string  `json:"sku"`
	Manufacturer string  `json:"manufacturer"`
	Category     string	 `json:"category"`
    Quantity    int     `json:"quantity"`
    CreatedAt   string  `json:"created_at"`
    UpdatedAt   string  `json:"updated_at"`
}

// ShoppingCart represents a complete shopping cart
type ShoppingCart struct {
    ID         int        `json:"id"`
    CustomerID int        `json:"customer_id"`
    Items      []CartItem `json:"items"`
    CreatedAt  string     `json:"created_at"`
    UpdatedAt  string     `json:"updated_at"`
}


// createShoppingCart creates a new shopping cart
// POST /shopping-carts
func createShoppingCart(c *gin.Context) {
    var input struct {
        CustomerID int `json:"customer_id" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "customer_id is required",
        })
        return
    }
    
    // Check if cart already exists for this customer
    var existingCartID int
    checkQuery := `SELECT id FROM shopping_carts WHERE customer_id = ?`
    err := DB.QueryRow(checkQuery, input.CustomerID).Scan(&existingCartID)
    
    if err == nil {
        // Cart already exists, return it
        c.JSON(http.StatusOK, gin.H{
            "message":     "Shopping cart already exists for this customer",
            "id":          existingCartID,
            "customer_id": input.CustomerID,
        })
        return
    }
    
    if err != sql.ErrNoRows {
        log.Printf("Error checking existing cart: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    
    // Insert new shopping cart
    query := `INSERT INTO shopping_carts (customer_id) VALUES (?)`
    result, err := DB.Exec(query, input.CustomerID)
    if err != nil {
        log.Printf("Error creating shopping cart: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to create shopping cart",
        })
        return
    }
    
    // Get the inserted cart ID
    cartID, err := result.LastInsertId()
    if err != nil {
        log.Printf("Error getting cart ID: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to retrieve cart ID",
        })
        return
    }
    
    // Return the created cart
    c.JSON(http.StatusCreated, gin.H{
        "id":          cartID,
        "customer_id": input.CustomerID,
        "message":     fmt.Sprintf("shopping cart %d created for customer %d", input.CustomerID, cartID),
        "created_at":  time.Now().Format(time.RFC3339),
    })
}

// getShoppingCart retrieves a shopping cart with all items by customer ID
// GET /shopping-carts/:id (where id is customer_id)
func getShoppingCart(c *gin.Context) {
    customerIDParam := c.Param("id")
    
    // Convert to integer
    customerID, err := strconv.Atoi(customerIDParam)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid customer ID",
        })
        return
    }
    
    // Get cart details by customer_id
    var cart ShoppingCart
    cartQuery := `SELECT id, customer_id, created_at, updated_at
                  FROM shopping_carts WHERE customer_id = ?`
    
    err = DB.QueryRow(cartQuery, customerID).Scan(
        &cart.ID,
        &cart.CustomerID,
		&cart.CreatedAt,
    	&cart.UpdatedAt,
    )
    
    if err == sql.ErrNoRows {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Shopping cart not found for this customer",
        })
        return
    }
    
    if err != nil {
        log.Printf("Database error retrieving cart: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    
    // Get cart items with product details using efficient JOINs
    itemsQuery := `
        SELECT 
            sci.id,
            sci.product_id,
			p.manufacturer,
			p.category,
            sci.quantity,
            sci.created_at,
            sci.updated_at
        FROM shopping_cart_items sci
        INNER JOIN products p ON sci.product_id = p.id
        WHERE sci.shopping_cart_id = ?
        ORDER BY sci.created_at DESC`
    
    rows, err := DB.Query(itemsQuery, cart.ID)
    if err != nil {
        log.Printf("Database error retrieving cart items: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    defer rows.Close()
    
    // Collect all items
    cart.Items = []CartItem{}
    for rows.Next() {
        var item CartItem
        err := rows.Scan(
            &item.ID,
            &item.ProductID,
            &item.Manufacturer,
            &item.Category,
            &item.Quantity,
            &item.CreatedAt,
            &item.UpdatedAt,
        )
        if err != nil {
            log.Printf("Error scanning cart item: %v", err)
            continue
        }
        cart.Items = append(cart.Items, item)
    }
    
    // Return the cart with all items
    c.JSON(http.StatusOK, cart)
}

// addItemToCart adds or updates an item in the shopping cart by customer ID
// POST /shopping-carts/:id/items (where id is customer_id)
func addItemToCart(c *gin.Context) {
    customerIDParam := c.Param("id")
    
    // Convert to integer
    customerID, err := strconv.Atoi(customerIDParam)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid customer ID",
        })
        return
    }
    
    // Parse request body
    var input struct {
        ProductID int `json:"product_id" binding:"required"`
        Quantity  int `json:"quantity" binding:"required,min=1"`
    }
    
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "product_id and quantity (min 1) are required",
        })
        return
    }
    
    // Get cart ID from customer_id
    var cartID int
    getCartQuery := `SELECT id FROM shopping_carts WHERE customer_id = ?`
    err = DB.QueryRow(getCartQuery, customerID).Scan(&cartID)
    
    if err == sql.ErrNoRows {
        c.JSON(http.StatusNotFound, gin.H{
            "error": "Shopping cart not found for this customer",
        })
        return
    }
    
    if err != nil {
        log.Printf("Error finding cart: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    
    // Verify product exists
    var productExists bool
    checkProductQuery := `SELECT EXISTS(SELECT 1 FROM products WHERE id = ?)`
    err = DB.QueryRow(checkProductQuery, input.ProductID).Scan(&productExists)
    if err != nil {
        log.Printf("Error checking product existence: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Internal server error",
        })
        return
    }
    
    if !productExists {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Product not found",
        })
        return
    }
    
    // Insert or update cart item (MySQL handles duplicate with ON DUPLICATE KEY UPDATE)
    insertQuery := `
        INSERT INTO shopping_cart_items (shopping_cart_id, product_id, quantity)
        VALUES (?, ?, ?)
        ON DUPLICATE KEY UPDATE 
            quantity = VALUES(quantity),
            updated_at = CURRENT_TIMESTAMP`
    
    result, err := DB.Exec(insertQuery, cartID, input.ProductID, input.Quantity)
    if err != nil {
        log.Printf("Error adding item to cart: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": "Failed to add item to cart",
        })
        return
    }
    
    // Check if it was an insert or update
    rowsAffected, _ := result.RowsAffected()
    
    // Get the item details to return
    var item CartItem
    itemQuery := `
        SELECT 
            sci.id,
            sci.product_id,
            p.manufacturer,
            p.category,
            sci.quantity,
            sci.created_at,
            sci.updated_at
        FROM shopping_cart_items sci
        INNER JOIN products p ON sci.product_id = p.id
        WHERE sci.shopping_cart_id = ? AND sci.product_id = ?`
    
    err = DB.QueryRow(itemQuery, cartID, input.ProductID).Scan(
        &item.ID,
        &item.ProductID,
        &item.Manufacturer,
        &item.Category,
        &item.Quantity,
        &item.CreatedAt,
        &item.UpdatedAt,
    )
    
    if err != nil {
        log.Printf("Error retrieving added item: %v", err)
        // Still return success since item was added
        c.JSON(http.StatusOK, gin.H{
            "message":    "Item added to cart",
            "product_id": input.ProductID,
            "quantity":   input.Quantity,
        })
        return
    }
    
    statusCode := http.StatusOK
    if rowsAffected == 1 {
        statusCode = http.StatusCreated
    }
    
    c.JSON(statusCode, gin.H{
        "message": "Item added to cart successfully",
        "item":    item,
    })
}



func searchProducts(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "INTERNAL_SERVER_ERROR",
				"message": "something went wrong",
				"details": fmt.Sprintf("%v", r),
			})
		}
	}()
	startTime := time.Now()

	// Extract query parameter
	query := c.Query("q")
	if query == "" {
		c.JSON(400, gin.H{"error": "Query parameter 'q' is required"})
		return
	}
	// Convert query to lowercase for case-insensitive search
	queryLower := strings.ToLower(query)

	// Generate 100 random product IDs (1-100000)
	randomIDs := generateRandomIDs(100, 1, 100000)

	// Search for matching products
	var matchingProducts []Item
	totalFound := 0
	totalSearched := 0

	for _, productID := range randomIDs {
		// Check if product exists in map
		totalSearched++
		if value, exists := syncProducts.Load(productID); exists {
			// Check if query matches name, category, or brand (case-insensitive)
			item := value.(Item)
			nameLower := strings.ToLower(item.Name)
			categoryLower := strings.ToLower(item.Category)
			brandLower := strings.ToLower(item.Brand)

			if strings.Contains(nameLower, queryLower) ||
				strings.Contains(categoryLower, queryLower) ||
				strings.Contains(brandLower, queryLower) {

				totalFound++

				// Add to results if we haven't reached 20 items yet
				if len(matchingProducts) < 20 {
					matchingProducts = append(matchingProducts, item)
				}
			}
		}
	}

	// Calculate search duration
	duration := time.Since(startTime)
	searchTime := fmt.Sprintf("%.3fs", duration.Seconds())

	// Create response
	response := SearchResponse{
		Products:      matchingProducts,
		TotalFound:    totalFound,
		TotalSearched: totalSearched,
		SearchTime:    searchTime,
	}

	// Return empty array instead of null if no products found
	if response.Products == nil {
		response.Products = []Item{}
	}

	c.JSON(200, response)
}

// generateRandomIDs generates n random integers between min and max (inclusive)
func generateRandomIDs(n, min, max int) []int {
	ids := make([]int, n)
	for i := 0; i < n; i++ {
		ids[i] = rand.Intn(max-min+1) + min
	}
	return ids
}

// postAlbums adds an album from JSON received in the request body.
func postItem(c *gin.Context) {

	defer func() {
		if r := recover(); r != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "INTERNAL_SERVER_ERROR",
				"message": "something went wrong",
				"details": fmt.Sprintf("%v", r),
			})
		}
	}()

	// Extract product ID from route
	productIDStr := c.Param("productId")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "data input invalid",
			"details": "invalid productId",
		})
		return
	}

	// Check if product exists in map
	_, exists := syncProducts.Load(productID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "NOT_FOUND",
			"message": "product not found",
			"details": fmt.Sprintf("no item with ID %d", productID),
		})
		return
	}

	// Call BindJSON to bind the received JSON (from request body) to
	// newItem.
	// if err := c.BindJSON(&newItem); err != nil {
	// 	return
	// }
	var newDetails Item
	if err := c.ShouldBindJSON(&newDetails); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "The provided input data is invalid",
			"details": err.Error(), // tells why decoding failed
		})
		return
	}

	// Ensure the product ID in body matches the route parameter
	if newDetails.ID != productID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "data input invalid",
			"details": "product_id in body does not match route parameter",
		})
		return
	}

	// Add the new details to the corresponding product.
	syncProducts.Store(productID, newDetails)

	c.Status(http.StatusNoContent)
}

// getItemByID locates the item whose ID value matches the productId
// parameter sent by the client, then returns that item as a response.
func getItemByID(c *gin.Context) {

	defer func() {
		if r := recover(); r != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "INTERNAL_SERVER_ERROR",
				"message": "something went wrong",
				"details": fmt.Sprintf("%v", r),
			})
		}
	}()

	// id := c.Param("productId") // "Context.Param()" retrieves the productId path parameter from the URL

	// Extract product ID from route
	productIDStr := c.Param("productId")
	productID, err := strconv.Atoi(productIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_INPUT",
			"message": "data input invalid",
			"details": "invalid productID",
		})
		return
	}
	// Check if product exists in map
	value, exists := syncProducts.Load(productID)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "INVALID_INPUT",
			"message": "product not found",
			"details": fmt.Sprintf("no item with ID %d", productID),
		})
		return
	}

	// return "404 not found error" if the album is not found
	c.IndentedJSON(http.StatusOK, value.(Item))

}
