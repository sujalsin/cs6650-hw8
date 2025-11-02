package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    "strings"
    
    _ "github.com/go-sql-driver/mysql"
)

// Global database connection pool
var DB *sql.DB

// InitDatabase initializes the database connection and applies schema
func InitDatabase() error {
    if err := connectDB(); err != nil {
        return fmt.Errorf("failed to connect to database: %w", err)
    }
    
    if err := runSchemaFromFile("./schema.sql"); err != nil {
        return fmt.Errorf("failed to apply schema: %w", err)
    }
    
    log.Println("Database initialized successfully")
    return nil
}

// connectDB establishes connection to MySQL database
func connectDB() error {
    var err error
    
    // Build connection string from environment variables
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
        os.Getenv("DB_USER"),
        os.Getenv("DB_PASSWORD"),
        os.Getenv("DB_HOST"),
        os.Getenv("DB_PORT"),
        os.Getenv("DB_NAME"),
    )
    
    // Open database connection
    DB, err = sql.Open("mysql", dsn)
    if err != nil {
        return fmt.Errorf("error opening database: %w", err)
    }
    
    // Test the connection
    if err = DB.Ping(); err != nil {
        return fmt.Errorf("error connecting to database: %w", err)
    }
    
    // Configure connection pool
    DB.SetMaxOpenConns(25)
    DB.SetMaxIdleConns(5)
    
    log.Println("Successfully connected to database")
    return nil
}

// runSchemaFromFile reads and executes SQL schema from file
func runSchemaFromFile(filename string) error {
    log.Printf("Reading schema from file: %s", filename)
    
    // Read the schema file
    schemaBytes, err := os.ReadFile(filename)
    if err != nil {
        return fmt.Errorf("error reading schema file %s: %w", filename, err)
    }
    
    schemaSQL := string(schemaBytes)
    
    // Remove comments and clean up the SQL
    schemaSQL = removeComments(schemaSQL)
    
    // Split by semicolons to get individual statements
    statements := strings.Split(schemaSQL, ";")
    
    executedCount := 0
    for i, stmt := range statements {
        stmt = strings.TrimSpace(stmt)
        
        // Skip empty statements
        if stmt == "" {
            continue
        }
        
        // Execute the statement
        log.Printf("Executing statement %d...", i+1)
        _, err := DB.Exec(stmt)
        if err != nil {
            return fmt.Errorf("error executing statement %d: %w\nStatement: %s", i+1, err, stmt)
        }
        executedCount++
    }
    
    log.Printf("Successfully executed %d SQL statements from schema file", executedCount)
    return nil
}

// removeComments removes SQL comments from the query
func removeComments(sql string) string {
    lines := strings.Split(sql, "\n")
    var cleaned []string
    
    for _, line := range lines {
        // Remove single-line comments (-- comments)
        if idx := strings.Index(line, "--"); idx >= 0 {
            line = line[:idx]
        }
        
        // Remove inline comments (# comments)
        if idx := strings.Index(line, "#"); idx >= 0 {
            line = line[:idx]
        }
        
        line = strings.TrimSpace(line)
        if line != "" {
            cleaned = append(cleaned, line)
        }
    }
    
    result := strings.Join(cleaned, " ")
    
    // Remove multi-line comments (/* */ comments)
    for {
        start := strings.Index(result, "/*")
        if start == -1 {
            break
        }
        end := strings.Index(result[start:], "*/")
        if end == -1 {
            break
        }
        result = result[:start] + result[start+end+2:]
    }
    
    return result
}

// CloseDatabase closes the database connection
func CloseDatabase() error {
    if DB != nil {
        log.Println("Closing database connection...")
        return DB.Close()
    }
    return nil
}



// Alternative: Batch insert for much better performance with large datasets
func seedProductsBatch(products map[int]Item) error {
	// Get lock (only one task can seed)
    _, err := DB.Exec("SELECT GET_LOCK('seed_lock', 30)")
    if err != nil {
        return err
    }
    defer DB.Exec("SELECT RELEASE_LOCK('seed_lock')")
    
    // Check if products already exist
    var count int
    e := DB.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
    if e != nil {
        return e
    }
    
    if count > 0 {
        log.Printf("Database already has %d products, skipping seed", count)
        return nil
    }
    
    log.Printf("Seeding %d products into database (batch mode)...", len(products))
    
    // Batch size
    batchSize := 1000
    
    values := make([]interface{}, 0, batchSize*10) // 10 fields per product
    insertedCount := 0
    
    for id := 1; id <= len(products); id++ {
        item, exists := products[id]
        if !exists {
            continue
        }
        
        values = append(values,
            item.ID,
            item.SKU,
            item.Manufacturer,
            item.CategoryID,
            item.Weight,
            item.SomeOtherID,
            item.Name,
            item.Category,
            item.Description,
            item.Brand,
        )
        
        // Execute batch when we reach batchSize
        if len(values) >= batchSize*10 {
            if err := executeBatchInsert(values, len(values)/10); err != nil {
                log.Printf("Error in batch insert: %v", err)
                return err
            }
            insertedCount += len(values) / 10
            log.Printf("Inserted %d products...", insertedCount)
            values = values[:0] // Clear slice
        }
    }
    
    // Insert remaining products
    if len(values) > 0 {
        if err := executeBatchInsert(values, len(values)/10); err != nil {
            log.Printf("Error in final batch insert: %v", err)
            return err
        }
        insertedCount += len(values) / 10
    }
    
    log.Printf("Seeding complete: %d products inserted", insertedCount)
    return nil
}

// executeBatchInsert performs a bulk insert
func executeBatchInsert(values []interface{}, numRows int) error {
    // Build query with correct number of placeholders
    valueStrings := make([]string, 0, numRows)
    for i := 0; i < numRows; i++ {
        valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
    }
    
    query := `INSERT INTO products (id, sku, manufacturer, category_id, weight, some_other_id, name, category, description, brand) VALUES `
    query += strings.Join(valueStrings, ",")
    
    _, err := DB.Exec(query, values...)
    return err
}