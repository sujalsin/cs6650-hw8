output "table_name" {
  description = "DynamoDB table name"
  value       = aws_dynamodb_table.shopping_carts.name
}

output "table_arn" {
  description = "DynamoDB table ARN"
  value       = aws_dynamodb_table.shopping_carts.arn
}

output "gsi_name" {
  description = "Global Secondary Index name for customer_id"
  value       = "customer_id-index"
}

