package storage

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/stretchr/testify/require"
)

func TestDynamo(t *testing.T) {
	runStorageTests(t, createDynamo)
}

// docker run -it --rm -p 8000:8000 amazon/dynamodb-local
// noinspection ALL
func createDynamo(t *testing.T) storage {
	d, err := NewDynamo(&aws.Config{
		Region:   aws.String("us-east-1"),
		Endpoint: aws.String("http://localhost:8000/"),
	})
	require.NoError(t, err)

	d.dynamo.DeleteTable(&dynamodb.DeleteTableInput{TableName: d.PledgesTableName})
	d.dynamo.DeleteTable(&dynamodb.DeleteTableInput{TableName: d.FeedsTableName})

	// Create Pledges table
	_, err = d.dynamo.CreateTable(&dynamodb.CreateTableInput{
		TableName: d.PledgesTableName,
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(pledgesPrimaryKey),
				AttributeType: aws.String("N"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(pledgesPrimaryKey),
				KeyType:       aws.String("HASH"),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	})

	require.NoError(t, err)

	// Create Feeds table
	_, err = d.dynamo.CreateTable(&dynamodb.CreateTableInput{
		TableName: d.FeedsTableName,
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String(feedsPrimaryKey),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("UserID"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("CreatedAt"),
				AttributeType: aws.String("N"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String(feedsPrimaryKey),
				KeyType:       aws.String("HASH"),
			},
		},
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: feedDowngradeIndexName,
				KeySchema: []*dynamodb.KeySchemaElement{
					{
						AttributeName: aws.String("UserID"),
						KeyType:       aws.String("HASH"),
					},
					{
						AttributeName: aws.String("CreatedAt"),
						KeyType:       aws.String("RANGE"),
					},
				},
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String("KEYS_ONLY"),
				},
				ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(1),
					WriteCapacityUnits: aws.Int64(1),
				},
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
	})

	require.NoError(t, err)

	err = d.dynamo.WaitUntilTableExists(&dynamodb.DescribeTableInput{TableName: d.PledgesTableName})
	require.NoError(t, err)

	err = d.dynamo.WaitUntilTableExists(&dynamodb.DescribeTableInput{TableName: d.FeedsTableName})
	require.NoError(t, err)

	_, err = d.dynamo.UpdateTimeToLive(&dynamodb.UpdateTimeToLiveInput{
		TableName: d.FeedsTableName,
		TimeToLiveSpecification: &dynamodb.TimeToLiveSpecification{
			AttributeName: feedTimeToLiveField,
			Enabled:       aws.Bool(true),
		},
	})

	require.NoError(t, err)

	return d
}