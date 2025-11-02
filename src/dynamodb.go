package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Global DynamoDB client
var DynamoDBClient *dynamodb.Client

// DynamoDB table name (will be set during initialization)
var DynamoDBTableName string

// GSI name for customer_id lookups
const CustomerIDIndexName = "customer_id-index"

// InitDynamoDB initializes the DynamoDB client using AWS SDK v2
func InitDynamoDB() error {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2" // Default region
	}

	// Load AWS SDK config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create DynamoDB client
	DynamoDBClient = dynamodb.NewFromConfig(cfg)

	// Set table name from environment or use default pattern
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "cs6650l2" // Default service name
	}
	DynamoDBTableName = fmt.Sprintf("%s-shopping-carts", serviceName)

	log.Printf("DynamoDB client initialized for table: %s (region: %s)", DynamoDBTableName, region)

	// Verify table exists by describing it
	_, err = DynamoDBClient.DescribeTable(context.TODO(), &dynamodb.DescribeTableInput{
		TableName: aws.String(DynamoDBTableName),
	})
	if err != nil {
		return fmt.Errorf("failed to verify DynamoDB table '%s': %w. Make sure the table exists and IAM permissions are correct", DynamoDBTableName, err)
	}

	log.Println("DynamoDB table verified successfully")
	return nil
}

// CloseDynamoDB closes the DynamoDB client (if needed)
func CloseDynamoDB() error {
	// DynamoDB client doesn't need explicit closing in SDK v2
	log.Println("DynamoDB client closed")
	return nil
}

// Helper function to check if DynamoDB error is a ResourceNotFoundException
func isResourceNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFoundErr *types.ResourceNotFoundException
	// Try to unwrap and check
	_, ok := err.(*types.ResourceNotFoundException)
	if ok {
		return true
	}
	// Check if wrapped
	var targetErr *types.ResourceNotFoundException
	if notFoundErr == targetErr {
		return false
	}
	return false
}

