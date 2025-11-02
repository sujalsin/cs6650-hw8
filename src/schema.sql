-- ============================================
-- PRODUCTS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS products (
  id INT AUTO_INCREMENT PRIMARY KEY,
  sku VARCHAR(100) NOT NULL UNIQUE,
  manufacturer VARCHAR(200),
  category_id INT,
  weight FLOAT,
  some_other_id INT,
  name VARCHAR(200),
  category VARCHAR(100),
  description TEXT,
  brand VARCHAR(100),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;

-- ============================================
-- SHOPPING CARTS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS shopping_carts (
  id INT AUTO_INCREMENT PRIMARY KEY,
  customer_id INT NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_customer_id (customer_id)
) ENGINE=InnoDB;

-- ============================================
-- SHOPPING CART ITEMS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS shopping_cart_items (
  id INT AUTO_INCREMENT PRIMARY KEY,
  shopping_cart_id INT NOT NULL,
  product_id INT NOT NULL,
  quantity INT NOT NULL CHECK (quantity > 0),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  
  -- Relationships
  CONSTRAINT fk_cart FOREIGN KEY (shopping_cart_id)
    REFERENCES shopping_carts (id)
    ON DELETE CASCADE,
  
  CONSTRAINT fk_product FOREIGN KEY (product_id)
    REFERENCES products (id)
    ON DELETE RESTRICT,
  
  -- Prevent duplicate product entries in the same cart
  UNIQUE KEY uq_cart_product (shopping_cart_id, product_id),
  
  -- Performance indexes
  INDEX idx_cart_id (shopping_cart_id),
  INDEX idx_product_id (product_id)
) ENGINE=InnoDB;