variable "service_name" {
  type        = string
  description = "Base name for RDS resources"
}

variable "vpc_id" {
  type        = string
  description = "VPC ID where RDS will be deployed"
}

variable "subnet_ids" {
  type        = list(string)
  description = "Private subnet IDs for RDS (should be at least 2 in different AZs)"
}

variable "ecs_security_group_ids" {
  type        = list(string)
  description = "Security group IDs of ECS tasks that need DB access"
}

variable "database_name" {
  type        = string
  description = "Initial database name"
  default     = "ecommerce"
}

variable "database_username" {
  type        = string
  description = "Master username for the database"
  default     = "admin"
}

variable "database_password" {
  type        = string
  description = "Master password for the database"
  sensitive   = true
}