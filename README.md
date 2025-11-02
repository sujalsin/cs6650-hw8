# CS6650 Homework 8 - Shopping Cart Data Layer

## AWS Configuration

### Step 1: Configure AWS Credentials

If you don't have AWS credentials configured, set them up:

```bash
aws configure
```

Enter:
- AWS Access Key ID: [Your access key]
- AWS Secret Access Key: [Your secret key]
- Default region name: `us-west-2`
- Default output format: `json`

### Step 2: Set Session Token (If Required)

If your AWS account requires session tokens (temporary credentials), you need to set the session token:

```bash
export AWS_SESSION_TOKEN="your-session-token-here"
```

Or add it to your AWS credentials file (`~/.aws/credentials`):

```ini
[default]
aws_access_key_id = YOUR_ACCESS_KEY
aws_secret_access_key = YOUR_SECRET_KEY
aws_session_token = YOUR_SESSION_TOKEN
region = us-west-2
```

### Step 3: Verify AWS Configuration

Verify your AWS configuration is correct:

```bash
aws sts get-caller-identity
```

This should return your AWS account ID and user/role information.

### Step 4: Verify LabRole Exists

The deployment requires an IAM role named "LabRole". Verify it exists:

```bash
aws iam get-role --role-name LabRole
```

## Deployment

### Deploying MySQL Backend (Step I)

1. Navigate to the terraform directory:

```bash
cd terraform
```

2. Initialize Terraform (first time only):

```bash
terraform init
```

3. Deploy with MySQL backend:

```bash
terraform apply -var="database_type=mysql"
```

4. Terraform will prompt for confirmation. Type `yes` to proceed.

5. Wait for deployment to complete (approximately 5-10 minutes). This creates:
   - VPC and networking resources
   - RDS MySQL instance
   - ECR repository
   - ECS cluster and service
   - Application Load Balancer
   - CloudWatch log groups

6. After deployment completes, get the application URL:

```bash
terraform output application_url
```

Note the URL (e.g., `http://cs6650l2-alb-xxxxx.us-west-2.elb.amazonaws.com`)

7. Wait 2-3 minutes for the ECS tasks to start and become healthy, then verify:

```bash
curl $(terraform output -raw application_url)/health
```

Expected response: `{"database":"mysql","status":"healthy"}`

### Deploying DynamoDB Backend (Step II)

1. If switching from MySQL, you can destroy the current deployment (optional):

```bash
terraform destroy -var="database_type=mysql"
```

2. Deploy with DynamoDB backend:

```bash
terraform apply -var="database_type=dynamodb"
```

3. Type `yes` to confirm.

4. Wait for deployment to complete (approximately 5-10 minutes). This creates:
   - All infrastructure from MySQL deployment
   - DynamoDB table: `cs6650l2-shopping-carts`
   - DynamoDB Global Secondary Index: `customer_id-index`

5. Get the application URL:

```bash
terraform output application_url
```

Note the URL (it may be different from MySQL deployment if ALB was recreated)

6. Wait 2-3 minutes for ECS tasks to become healthy, then verify:

```bash
curl $(terraform output -raw application_url)/health
```

Expected response: `{"database":"dynamodb","status":"healthy"}`

### Switching Between MySQL and DynamoDB

You can switch between backends without destroying everything:

```bash
# Switch to MySQL
terraform apply -var="database_type=mysql"

# Switch to DynamoDB
terraform apply -var="database_type=dynamodb"
```

Terraform will update the ECS task definition to use the correct backend. Wait 2-3 minutes for the new tasks to become healthy.

## Running Tests

### Prerequisites for Testing

Before running tests, ensure you have the application URL from Terraform outputs.

### Test MySQL Backend

1. Get the current application URL:

```bash
cd terraform
APPLICATION_URL=$(terraform output -raw application_url)
echo $APPLICATION_URL
```

2. Update the test file with the correct URL:

Edit `testing/test.go` and update line 18 with your application URL:

```go
BaseURL = "http://YOUR-ALB-URL.us-west-2.elb.amazonaws.com"
```

3. Navigate to testing directory:

```bash
cd ../testing
```

4. Run the MySQL test:

```bash
go run test.go
```

5. The test will:
   - Create 50 shopping carts
   - Add items to 50 carts
   - Retrieve 50 carts
   - Generate `test_results.json` with results

6. Check the results:

```bash
cat test_results.json | jq '.statistics'
```

### Test DynamoDB Backend

1. Get the current application URL:

```bash
cd terraform
APPLICATION_URL=$(terraform output -raw application_url)
echo $APPLICATION_URL
```

2. Update the test file with the correct URL:

Edit `testing/test_dynamodb.go` and update line 18 with your application URL:

```go
BaseURL = "http://YOUR-ALB-URL.us-west-2.elb.amazonaws.com"
```

3. Navigate to testing directory:

```bash
cd ../testing
```

4. Run the DynamoDB test:

```bash
go run test_dynamodb.go
```

5. The test will:
   - Create 50 shopping carts
   - Add items to 50 carts
   - Retrieve 50 carts
   - Generate `dynamodb_test_results.json` with results

6. Check the results:

```bash
cat dynamodb_test_results.json | jq '.statistics'
```

## Quick Test URL Update Script

Instead of manually editing the test files, you can use this script to update both test files:

```bash
#!/bin/bash
cd terraform
URL=$(terraform output -raw application_url)
cd ../testing
sed -i '' "s|BaseURL.*=.*\"http.*\"|BaseURL = \"$URL\"|" test.go
sed -i '' "s|BaseURL.*=.*\"http.*\"|BaseURL = \"$URL\"|" test_dynamodb.go
echo "Updated test files with URL: $URL"
```

Save this as `update_test_url.sh`, make it executable (`chmod +x update_test_url.sh`), and run it whenever the ALB URL changes.

## Project Structure

```
CS6650-HW8/
├── src/                    # Go application source code
│   ├── main.go             # Application entry point
│   ├── handlers.go         # HTTP handlers (MySQL and DynamoDB)
│   ├── database.go         # MySQL database connection
│   ├── dynamodb.go         # DynamoDB client initialization
│   └── Dockerfile          # Docker build configuration
├── terraform/              # Infrastructure as Code
│   ├── main.tf             # Main Terraform configuration
│   ├── variables.tf        # Variable definitions
│   └── modules/            # Terraform modules
│       ├── alb/            # Application Load Balancer
│       ├── dynamodb/       # DynamoDB table
│       ├── ecr/            # Elastic Container Registry
│       ├── ecs/            # Elastic Container Service
│       ├── logging/        # CloudWatch Logs
│       ├── network/        # VPC and networking
│       └── rds/            # RDS MySQL instance
└── testing/                # Test scripts and results
    ├── test.go             # MySQL test script
    ├── test_dynamodb.go    # DynamoDB test script
    ├── test_results.json   # MySQL test results
    └── dynamodb_test_results.json  # DynamoDB test results
```

## Troubleshooting

### AWS Session Token Expired

If you get authentication errors, your session token may have expired. Re-authenticate:

```bash
# Re-run AWS configure or update session token
export AWS_SESSION_TOKEN="new-session-token"
```

### ECS Tasks Not Starting

1. Check ECS service status:

```bash
aws ecs describe-services --cluster cs6650l2-cluster --services cs6650l2
```

2. Check CloudWatch logs:

```bash
aws logs tail $(aws logs describe-log-groups --query 'logGroups[?contains(logGroupName, `cs6650l2`)].logGroupName' --output text | head -1) --follow
```

### Health Check Failing

1. Verify the application is running:

```bash
curl $(terraform output -raw application_url)/health
```

2. Check ECS task logs for errors:

```bash
aws ecs list-tasks --cluster cs6650l2-cluster --service-name cs6650l2
# Use task ID from above to get logs
```

### Test Failures

1. Verify the ALB URL is correct in test files
2. Check if the service is healthy: `curl $URL/health`
3. Verify database type matches deployment (mysql vs dynamodb)
4. Check CloudWatch logs for application errors

### RDS Already Exists Error

If you get "DB instance already exists" error:

```bash
# Refresh Terraform state
terraform refresh -var="database_type=dynamodb"

# Or manually import if needed
terraform import module.rds.aws_db_instance.this cs6650l2-db
```

## Cleanup

To destroy all resources:

```bash
cd terraform
terraform destroy  # or mysql
```

**Warning**: This will delete all AWS resources including databases. Make sure you have backups if needed.

## Additional Notes

- The MySQL backend uses RDS for both shopping carts and products
- The DynamoDB backend uses DynamoDB for shopping carts but still uses MySQL for products (hybrid approach)
- Both implementations maintain API compatibility - same endpoints and request/response formats
- The application automatically detects the database type from the `DATABASE_TYPE` environment variable
- ECS tasks use the `LabRole` IAM role which must have permissions for:
  - DynamoDB (read/write)
  - RDS access (for products)
  - CloudWatch Logs (logging)
  - ECR (pulling images)

## Support

For issues or questions, check:
- Terraform state: `terraform show`
- AWS Console: ECS, RDS, DynamoDB, CloudWatch
- Application logs: CloudWatch Logs groups