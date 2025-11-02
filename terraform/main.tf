# Wire together four focused modules: network, ecr, logging, ecs.

module "network" {
  source         = "./modules/network"
  service_name   = var.service_name
  container_port = var.container_port
}

module "ecr" {
  source          = "./modules/ecr"
  repository_name = var.ecr_repository_name
}

# Application Load Balancer
module "alb" {
  source         = "./modules/alb"
  service_name   = var.service_name
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.subnet_ids
  container_port = var.container_port
}

module "logging" {
  source            = "./modules/logging"
  service_name      = var.service_name
  retention_in_days = var.log_retention_days
}

# RDS MySQL Database
module "rds" {
  source                 = "./modules/rds"
  service_name           = var.service_name
  vpc_id                 = module.network.vpc_id
  subnet_ids             = module.network.subnet_ids
  ecs_security_group_ids = [module.network.security_group_id]
  database_name          = var.database_name
  database_username      = var.database_username
  database_password      = var.database_password
}

# DynamoDB Table for Shopping Carts
module "dynamodb" {
  source       = "./modules/dynamodb"
  service_name = var.service_name
}

# Reuse an existing IAM role for ECS tasks
data "aws_iam_role" "lab_role" {
  name = "LabRole"
}

module "ecs" {
  source             = "./modules/ecs"
  service_name       = var.service_name
  image              = "${module.ecr.repository_url}:latest"
  container_port     = var.container_port
  subnet_ids         = module.network.subnet_ids
  security_group_ids = [module.network.security_group_id]
  execution_role_arn = data.aws_iam_role.lab_role.arn
  task_role_arn      = data.aws_iam_role.lab_role.arn
  log_group_name     = module.logging.log_group_name
  ecs_count          = var.ecs_count
  region             = var.aws_region

  # ALB integration
  target_group_arn = module.alb.target_group_arn

  # Auto-scaling configuration
  min_capacity = var.min_capacity
  max_capacity = var.max_capacity

  # Pass DB connection info as environment variables
  db_host     = module.rds.db_instance_address
  db_port     = module.rds.db_instance_port
  db_name     = module.rds.db_name
  db_username = var.database_username
  db_password = var.database_password

  # DynamoDB configuration
  database_type = var.database_type
  aws_region    = var.aws_region
}


// Build & push the Go app image into ECR
resource "docker_image" "app" {
  # Use the URL from the ecr module, and tag it "latest"
  name = "${module.ecr.repository_url}:latest"

  build {
    # relative path from terraform/ → src/
    context = "../src"
    # Dockerfile defaults to "Dockerfile" in that context
  }
}

resource "docker_registry_image" "app" {
  # this will push :latest → ECR
  name = docker_image.app.name
}
