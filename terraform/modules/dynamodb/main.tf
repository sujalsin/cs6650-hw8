# DynamoDB Table for Shopping Carts
resource "aws_dynamodb_table" "shopping_carts" {
  name         = "${var.service_name}-shopping-carts"
  billing_mode = "PAY_PER_REQUEST" # On-demand billing

  # Partition key: cart_id (UUID string) for even distribution
  hash_key = "cart_id"

  # Attributes
  attribute {
    name = "cart_id"
    type = "S" # String
  }

  attribute {
    name = "customer_id"
    type = "N" # Number (to match MySQL customer_id as integer)
  }

  # Global Secondary Index for customer_id lookups
  # Required because API uses customer_id as {id} path parameter
  global_secondary_index {
    name            = "customer_id-index"
    hash_key        = "customer_id"
    projection_type = "ALL" # Project all attributes (needed for full cart retrieval)
  }

  tags = {
    Name        = "${var.service_name}-shopping-carts-dynamodb"
    Description = "Shopping carts table for DynamoDB implementation"
  }
}

