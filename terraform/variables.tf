# Region to deploy into
variable "aws_region" {
  type    = string
  default = "us-west-2"
}

# ECR & ECS settings
variable "ecr_repository_name" {
  type    = string
  default = "ecr_service"
}

variable "service_name" {
  type    = string
  default = "cs6650l2"
}

variable "container_port" {
  type    = number
  default = 8080
}

variable "ecs_count" {
  type    = number
  default = 1
}

# How long to keep logs
variable "log_retention_days" {
  type    = number
  default = 7
}

# Auto-scaling settings
variable "min_capacity" {
  type        = number
  default     = 1
  description = "Minimum number of ECS tasks"
}

variable "max_capacity" {
  type        = number
  default     = 10
  description = "Maximum number of ECS tasks"
}

# RDS Database settings
variable "database_name" {
  type        = string
  description = "Name of the initial database"
  default     = "ecommerce"
}

variable "database_username" {
  type        = string
  description = "Master username for RDS"
  default     = "admin"
}

variable "database_password" {
  type        = string
  description = "Master password for RDS"
  sensitive   = true
  default     = "TempPassword123!" # Default password (change for production)
}

# Database type selection
variable "database_type" {
  type        = string
  description = "Database type: 'mysql' or 'dynamodb'"
  default     = "mysql"
}