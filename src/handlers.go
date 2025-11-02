package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CartItem represents an item in the shopping cart
type CartItem struct {
	ID        int `json:"id"`
	ProductID int `json:"product_id"`
	// ProductName string  `json:"product_name"`
	// SKU         string  `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	Category     string `json:"category"`
	Quantity     int    `json:"quantity"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
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

// ============================================================================
// DynamoDB Handlers (separate implementations alongside MySQL handlers)
// ============================================================================

// createShoppingCartDynamoDB creates a new shopping cart using DynamoDB
// POST /shopping-carts
func createShoppingCartDynamoDB(c *gin.Context) {
	var input struct {
		CustomerID int `json:"customer_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "customer_id is required",
		})
		return
	}

	ctx := context.Background()

	// Check if cart already exists for this customer using GSI
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(DynamoDBTableName),
		IndexName:              aws.String(CustomerIDIndexName),
		KeyConditionExpression: aws.String("customer_id = :customer_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":customer_id": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", input.CustomerID)},
		},
		Limit: aws.Int32(1),
	}

	result, err := DynamoDBClient.Query(ctx, queryInput)
	if err != nil {
		// Check if it's a not found error or other error
		var notFoundErr *types.ResourceNotFoundException
		if errors.As(err, &notFoundErr) {
			// Index doesn't exist yet or no results - continue to create
		} else {
			log.Printf("Error querying cart by customer_id: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			return
		}
	}

	// If cart exists, return it (matching MySQL behavior)
	if result != nil && len(result.Items) > 0 {
		var cartIDInt int
		// First check for numeric_id field (preferred)
		if numericIDAttr, ok := result.Items[0]["numeric_id"]; ok {
			if numericIDMember, ok := numericIDAttr.(*types.AttributeValueMemberN); ok {
				cartIDInt, _ = strconv.Atoi(numericIDMember.Value)
			}
		}
		// If numeric_id not found, use a hash of cart_id UUID
		if cartIDInt == 0 {
			if cartIDAttr, ok := result.Items[0]["cart_id"]; ok {
				if cartIDMember, ok := cartIDAttr.(*types.AttributeValueMemberS); ok {
					// Use first few characters of UUID to generate numeric ID
					// This is a simple approach - in production you'd store numeric_id
					cartIDInt = int(cartIDMember.Value[0])*1000 + int(cartIDMember.Value[1])*10
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"message":     "Shopping cart already exists for this customer",
			"id":          cartIDInt,
			"customer_id": input.CustomerID,
		})
		return
	}

	// Generate UUID for partition key (cart_id)
	cartIDUUID := uuid.New().String()
	// Generate numeric ID for API response compatibility (use deterministic hash of UUID)
	// Using first 8 chars of UUID hex as numeric base
	cartIDInt := int(time.Now().UnixNano() % 100000000) // Use timestamp-based ID for simplicity

	// Create new cart
	now := time.Now().Format(time.RFC3339)
	putInput := &dynamodb.PutItemInput{
		TableName: aws.String(DynamoDBTableName),
		Item: map[string]types.AttributeValue{
			"cart_id":     &types.AttributeValueMemberS{Value: cartIDUUID},
			"numeric_id":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", cartIDInt)},
			"customer_id": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", input.CustomerID)},
			"cart_items":  &types.AttributeValueMemberL{Value: []types.AttributeValue{}}, // Empty items list
			"created_at":  &types.AttributeValueMemberS{Value: now},
			"updated_at":  &types.AttributeValueMemberS{Value: now},
		},
	}

	_, err = DynamoDBClient.PutItem(ctx, putInput)
	if err != nil {
		log.Printf("Error creating shopping cart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create shopping cart",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          cartIDInt,
		"customer_id": input.CustomerID,
		"message":     fmt.Sprintf("shopping cart %d created for customer %d", cartIDInt, input.CustomerID),
		"created_at":  now,
	})
}

// getShoppingCartDynamoDB retrieves a shopping cart with all items by customer ID
// GET /shopping-carts/:id (where id is customer_id)
func getShoppingCartDynamoDB(c *gin.Context) {
	customerIDParam := c.Param("id")

	// Convert to integer
	customerID, err := strconv.Atoi(customerIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid customer ID",
		})
		return
	}

	ctx := context.Background()

	// Query by customer_id using GSI
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(DynamoDBTableName),
		IndexName:              aws.String(CustomerIDIndexName),
		KeyConditionExpression: aws.String("customer_id = :customer_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":customer_id": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", customerID)},
		},
		Limit: aws.Int32(1),
	}

	result, err := DynamoDBClient.Query(ctx, queryInput)
	if err != nil {
		log.Printf("Error querying cart by customer_id: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	if len(result.Items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Shopping cart not found for this customer",
		})
		return
	}

	// Extract cart data
	item := result.Items[0]
	var cart ShoppingCart

	// Extract cart_id (numeric_id for API compatibility)
	if numericIDAttr, ok := item["numeric_id"]; ok {
		if numericIDMember, ok := numericIDAttr.(*types.AttributeValueMemberN); ok {
			cart.ID, _ = strconv.Atoi(numericIDMember.Value)
		}
	} else if cartIDAttr, ok := item["cart_id"]; ok {
		// Fallback to cart_id if numeric_id not found
		if cartIDMember, ok := cartIDAttr.(*types.AttributeValueMemberS); ok {
			// Try to parse as int (if stored as string representation)
			cart.ID, _ = strconv.Atoi(cartIDMember.Value)
		}
	}

	// Extract customer_id
	if customerIDAttr, ok := item["customer_id"]; ok {
		if customerIDMember, ok := customerIDAttr.(*types.AttributeValueMemberN); ok {
			cart.CustomerID, _ = strconv.Atoi(customerIDMember.Value)
		}
	}

	// Extract timestamps
	if createdAttr, ok := item["created_at"]; ok {
		if createdMember, ok := createdAttr.(*types.AttributeValueMemberS); ok {
			cart.CreatedAt = createdMember.Value
		}
	}
	if updatedAttr, ok := item["updated_at"]; ok {
		if updatedMember, ok := updatedAttr.(*types.AttributeValueMemberS); ok {
			cart.UpdatedAt = updatedMember.Value
		}
	}

	// Extract items list
	cart.Items = []CartItem{}
	if itemsAttr, ok := item["cart_items"]; ok {
		if itemsMember, ok := itemsAttr.(*types.AttributeValueMemberL); ok {
			for _, itemAttr := range itemsMember.Value {
				if itemMap, ok := itemAttr.(*types.AttributeValueMemberM); ok {
					var cartItem CartItem
					// Extract fields from item map
					if idAttr, ok := itemMap.Value["id"]; ok {
						if idMember, ok := idAttr.(*types.AttributeValueMemberN); ok {
							cartItem.ID, _ = strconv.Atoi(idMember.Value)
						}
					}
					if productIDAttr, ok := itemMap.Value["product_id"]; ok {
						if productIDMember, ok := productIDAttr.(*types.AttributeValueMemberN); ok {
							cartItem.ProductID, _ = strconv.Atoi(productIDMember.Value)
						}
					}
					if quantityAttr, ok := itemMap.Value["quantity"]; ok {
						if quantityMember, ok := quantityAttr.(*types.AttributeValueMemberN); ok {
							cartItem.Quantity, _ = strconv.Atoi(quantityMember.Value)
						}
					}
					if manufacturerAttr, ok := itemMap.Value["manufacturer"]; ok {
						if manufacturerMember, ok := manufacturerAttr.(*types.AttributeValueMemberS); ok {
							cartItem.Manufacturer = manufacturerMember.Value
						}
					}
					if categoryAttr, ok := itemMap.Value["category"]; ok {
						if categoryMember, ok := categoryAttr.(*types.AttributeValueMemberS); ok {
							cartItem.Category = categoryMember.Value
						}
					}
					if createdAtAttr, ok := itemMap.Value["created_at"]; ok {
						if createdAtMember, ok := createdAtAttr.(*types.AttributeValueMemberS); ok {
							cartItem.CreatedAt = createdAtMember.Value
						}
					}
					if updatedAtAttr, ok := itemMap.Value["updated_at"]; ok {
						if updatedAtMember, ok := updatedAtAttr.(*types.AttributeValueMemberS); ok {
							cartItem.UpdatedAt = updatedAtMember.Value
						}
					}
					cart.Items = append(cart.Items, cartItem)
				}
			}
		}
	}

	// Return the cart with all items (matching MySQL format)
	c.JSON(http.StatusOK, cart)
}

// addItemToCartDynamoDB adds or updates an item in the shopping cart by customer ID
// POST /shopping-carts/:id/items (where id is customer_id)
func addItemToCartDynamoDB(c *gin.Context) {
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

	ctx := context.Background()

	// Get cart by customer_id using GSI
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(DynamoDBTableName),
		IndexName:              aws.String(CustomerIDIndexName),
		KeyConditionExpression: aws.String("customer_id = :customer_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":customer_id": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", customerID)},
		},
		Limit: aws.Int32(1),
	}

	result, err := DynamoDBClient.Query(ctx, queryInput)
	if err != nil {
		log.Printf("Error querying cart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	if len(result.Items) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Shopping cart not found for this customer",
		})
		return
	}

	// Get cart_id from query result
	var cartID string
	if cartIDAttr, ok := result.Items[0]["cart_id"]; ok {
		if cartIDMember, ok := cartIDAttr.(*types.AttributeValueMemberS); ok {
			cartID = cartIDMember.Value
		}
	}

	// Validate cartID was extracted
	if cartID == "" {
		log.Printf("Error: cart_id not found in query result")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	// Verify product exists (still need MySQL for product lookup)
	// For now, we'll skip validation or use syncProducts map
	var productExists bool
	if value, exists := syncProducts.Load(input.ProductID); exists {
		productExists = true
		_ = value // Product found
	}

	if !productExists {
		// Try MySQL DB if available
		if DB != nil {
			var exists bool
			checkProductQuery := `SELECT EXISTS(SELECT 1 FROM products WHERE id = ?)`
			err = DB.QueryRow(checkProductQuery, input.ProductID).Scan(&exists)
			if err == nil && exists {
				productExists = true
			}
		}
	}

	if !productExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Product not found",
		})
		return
	}

	// Get product details for manufacturer and category
	var manufacturer, category string
	if value, exists := syncProducts.Load(input.ProductID); exists {
		product := value.(Item)
		manufacturer = product.Manufacturer
		category = product.Category
	} else if DB != nil {
		// Query MySQL for product details
		query := `SELECT manufacturer, category FROM products WHERE id = ?`
		err = DB.QueryRow(query, input.ProductID).Scan(&manufacturer, &category)
		if err != nil {
			log.Printf("Error getting product details: %v", err)
			manufacturer = ""
			category = ""
		}
	}

	// Get current cart to update items
	getInput := &dynamodb.GetItemInput{
		TableName: aws.String(DynamoDBTableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartID},
		},
	}

	cartResult, err := DynamoDBClient.GetItem(ctx, getInput)
	if err != nil {
		log.Printf("Error getting cart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	if cartResult.Item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Shopping cart not found",
		})
		return
	}

	// Extract existing items
	var existingItems []types.AttributeValue
	if itemsAttr, ok := cartResult.Item["cart_items"]; ok {
		if itemsMember, ok := itemsAttr.(*types.AttributeValueMemberL); ok {
			existingItems = itemsMember.Value
		}
	}

	// Check if item already exists and update, or add new
	now := time.Now().Format(time.RFC3339)
	foundIndex := -1
	for i, itemAttr := range existingItems {
		if itemMap, ok := itemAttr.(*types.AttributeValueMemberM); ok {
			if productIDAttr, ok := itemMap.Value["product_id"]; ok {
				if productIDMember, ok := productIDAttr.(*types.AttributeValueMemberN); ok {
					if productIDMember.Value == fmt.Sprintf("%d", input.ProductID) {
						foundIndex = i
						break
					}
				}
			}
		}
	}

	// Create/update item
	newItem := map[string]types.AttributeValue{
		"product_id":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", input.ProductID)},
		"quantity":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", input.Quantity)},
		"manufacturer": &types.AttributeValueMemberS{Value: manufacturer},
		"category":     &types.AttributeValueMemberS{Value: category},
		"updated_at":   &types.AttributeValueMemberS{Value: now},
	}

	// Generate item ID if new item
	if foundIndex == -1 {
		// New item - generate ID based on position
		itemID := len(existingItems) + 1
		newItem["id"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", itemID)}
		newItem["created_at"] = &types.AttributeValueMemberS{Value: now}
		existingItems = append(existingItems, &types.AttributeValueMemberM{Value: newItem})
	} else {
		// Update existing item
		if existingItemMap, ok := existingItems[foundIndex].(*types.AttributeValueMemberM); ok {
			// Preserve existing id and created_at
			if idAttr, ok := existingItemMap.Value["id"]; ok {
				newItem["id"] = idAttr
			}
			if createdAtAttr, ok := existingItemMap.Value["created_at"]; ok {
				newItem["created_at"] = createdAtAttr
			}
			existingItems[foundIndex] = &types.AttributeValueMemberM{Value: newItem}
		}
	}

	// Update cart with new items
	updateInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(DynamoDBTableName),
		Key: map[string]types.AttributeValue{
			"cart_id": &types.AttributeValueMemberS{Value: cartID},
		},
		UpdateExpression: aws.String("SET cart_items = :cart_items, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":cart_items": &types.AttributeValueMemberL{Value: existingItems},
			":updated_at": &types.AttributeValueMemberS{Value: now},
		},
		ReturnValues: types.ReturnValueAllNew,
	}

	updateResult, err := DynamoDBClient.UpdateItem(ctx, updateInput)
	if err != nil {
		log.Printf("Error updating cart: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to add item to cart",
		})
		return
	}

	// Build response item (matching MySQL format)
	var responseItem CartItem
	if foundIndex == -1 {
		// New item
		responseItem.ID = len(existingItems)
		responseItem.CreatedAt = now
	} else {
		// Updated item - extract from update result
		if itemsAttr, ok := updateResult.Attributes["cart_items"]; ok {
			if itemsMember, ok := itemsAttr.(*types.AttributeValueMemberL); ok && len(itemsMember.Value) > foundIndex {
				if itemMap, ok := itemsMember.Value[foundIndex].(*types.AttributeValueMemberM); ok {
					if idAttr, ok := itemMap.Value["id"]; ok {
						if idMember, ok := idAttr.(*types.AttributeValueMemberN); ok {
							responseItem.ID, _ = strconv.Atoi(idMember.Value)
						}
					}
					if createdAtAttr, ok := itemMap.Value["created_at"]; ok {
						if createdAtMember, ok := createdAtAttr.(*types.AttributeValueMemberS); ok {
							responseItem.CreatedAt = createdAtMember.Value
						}
					}
				}
			}
		}
	}
	responseItem.ProductID = input.ProductID
	responseItem.Quantity = input.Quantity
	responseItem.Manufacturer = manufacturer
	responseItem.Category = category
	responseItem.UpdatedAt = now

	statusCode := http.StatusOK
	if foundIndex == -1 {
		statusCode = http.StatusCreated
	}

	c.JSON(statusCode, gin.H{
		"message": "Item added to cart successfully",
		"item":    responseItem,
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
