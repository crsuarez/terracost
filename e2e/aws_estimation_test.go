package e2e

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	costestimation "github.com/cycloidio/terracost"
	"github.com/cycloidio/terracost/aws"
	"github.com/cycloidio/terracost/aws/region"
	awstf "github.com/cycloidio/terracost/aws/terraform"
	"github.com/cycloidio/terracost/cost"
	"github.com/cycloidio/terracost/mysql"
	"github.com/cycloidio/terracost/price"
	"github.com/cycloidio/terracost/product"
	"github.com/cycloidio/terracost/terraform"
	"github.com/cycloidio/terracost/usage"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	noModulePath            = ""
	noForceTerragrunt       bool
	noDebug                 bool
	noParallelismTerragrunt = 0
)

// terraformAWSTestProviderInitializer is a testing ProviderInitializer
// pricing are directly inserted inside the database, which allows us to
// test the processing with smaller subset of data, as well as the functioning
// of MatchNames for a given provider - as data are injected using 'aws-test'
// which is also used in the tfplan & co.
var terraformAWSTestProviderInitializer = terraform.ProviderInitializer{
	MatchNames: []string{"aws", "aws-test"},
	Provider: func(config map[string]interface{}) (terraform.Provider, error) {
		r, ok := config["region"]
		if !ok {
			return nil, nil
		}
		regCode := region.Code(r.(string))
		return awstf.NewProvider("aws-test", regCode)
	},
}

func TestAWSEstimation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx := context.Background()
	db, err := sql.Open("mysql", testDSN())
	require.NoError(t, err)

	backend := mysql.NewBackend(db)

	prods := []*product.Product{
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-T2-MICRO",
			Service:  "AmazonEC2",
			Family:   "Compute Instance",
			Location: "us-east-1",
			Attributes: map[string]string{
				"CapacityStatus":  "Used",
				"InstanceType":    "t2.micro",
				"Tenancy":         "Shared",
				"OperatingSystem": "Linux",
				"PreInstalledSW":  "NA",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-T2-XLARGE",
			Service:  "AmazonEC2",
			Family:   "Compute Instance",
			Location: "us-east-1",
			Attributes: map[string]string{
				"CapacityStatus":  "Used",
				"InstanceType":    "t2.xlarge",
				"Tenancy":         "Shared",
				"OperatingSystem": "Linux",
				"PreInstalledSW":  "NA",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-STORAGE",
			Service:  "AmazonEC2",
			Family:   "Storage",
			Location: "us-east-1",
			Attributes: map[string]string{
				"VolumeAPIName": "gp2",
			},
		},
		// Lambda products
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-LAMBDA-REQUESTS",
			Service:  "AWSLambda",
			Family:   "Serverless",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "AWS-Lambda-Requests",
				"UsageType": "Request",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-LAMBDA-DURATION-X86",
			Service:  "AWSLambda",
			Family:   "Serverless",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "AWS-Lambda-Duration",
				"UsageType": "Lambda-GB-Seconds",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-LAMBDA-DURATION-ARM",
			Service:  "AWSLambda",
			Family:   "Serverless",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "AWS-Lambda-Duration",
				"UsageType": "Lambda-ARM-GB-Seconds",
			},
		},
		// DynamoDB products
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-OD-READ",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-ReadUnits",
				"UsageType": "ReadRequestUnits",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-OD-WRITE",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-WriteUnits",
				"UsageType": "WriteRequestUnits",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-STORAGE",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-StorageUsage",
				"UsageType": "TimedStorage-ByteHrs",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-PITR",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-CrossRegionReplication",
				"UsageType": "DDB-PITRStorageSnapshotByteHrs",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-STREAM",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-StreamsEventDataUnits",
				"UsageType": "DDB-Streams-ReadUnits",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-PROV-RCU",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-ReadUnits",
				"UsageType": "ReadCapacityUnit-Hrs",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-DDB-PROV-WCU",
			Service:  "AmazonDynamoDB",
			Family:   "AmazonDynamoDB",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DDB-WriteUnits",
				"UsageType": "WriteCapacityUnit-Hrs",
			},
		},
		// API Gateway products
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-APIGW-REST-REQ",
			Service:  "AmazonApiGateway",
			Family:   "AmazonApiGateway",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "ApiGatewayRequest",
				"UsageType": "USE1-ApiGatewayRequest",
				"Operation": "ApiGatewayRestApi",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-APIGW-HTTP-REQ",
			Service:  "AmazonApiGateway",
			Family:   "AmazonApiGateway",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "ApiGatewayHttpRequest",
				"UsageType": "USE1-ApiGatewayHttpRequest",
				"Operation": "ApiGatewayHttpApi",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-APIGW-WS-MSG",
			Service:  "AmazonApiGateway",
			Family:   "AmazonApiGateway",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "ApiGatewayWebSocketMessage",
				"UsageType": "USE1-ApiGatewayMessage",
				"Operation": "ApiGatewayWebSocket",
			},
		},
		// CloudFront products
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-CF-DT-NA",
			Service:  "AmazonCloudFront",
			Family:   "Data Transfer",
			Location: "North America",
			Attributes: map[string]string{
				"Group": "CloudFront-DataTransfer-Out-Bytes",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-CF-DT-EU",
			Service:  "AmazonCloudFront",
			Family:   "Data Transfer",
			Location: "Europe",
			Attributes: map[string]string{
				"Group": "CloudFront-DataTransfer-Out-Bytes",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-CF-HTTPS",
			Service:  "AmazonCloudFront",
			Family:   "Request",
			Location: "North America",
			Attributes: map[string]string{
				"Group": "CloudFront-Requests-HTTPS-Proxy",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-CF-INVALIDATION",
			Service:  "AmazonCloudFront",
			Family:   "Invalidation",
			Location: "North America",
			Attributes: map[string]string{
				"Group": "CloudFront-Invalidation",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-CF-FUNC",
			Service:  "AmazonCloudFront",
			Family:   "CloudFront Functions",
			Location: "North America",
			Attributes: map[string]string{
				"Group":     "CloudFront-EdgeFunctions",
				"UsageType": "CloudFront-Function-Invocations",
			},
		},
		// Route 53 products
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-HOSTEDZONE",
			Service:  "AmazonRoute53",
			Family:   "DNS Zone",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group": "HostedZone",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-STD-QUERIES",
			Service:  "AmazonRoute53",
			Family:   "DNS Query",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DNS-Queries",
				"UsageType": "DNS-Queries",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-LAT-QUERIES",
			Service:  "AmazonRoute53",
			Family:   "DNS Query",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "DNS-Queries",
				"UsageType": "DNS-LatencyBasedRoutingQueries",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-HC-BASIC",
			Service:  "AmazonRoute53",
			Family:   "DNS Health Check",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group": "HealthCheck-AWS-Endpoint",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-HC-CUSTOM",
			Service:  "AmazonRoute53",
			Family:   "DNS Health Check",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group": "HealthCheck-Custom-Endpoint",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-HC-FEAT-STRMATCH",
			Service:  "AmazonRoute53",
			Family:   "DNS Health Check",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "HealthCheck-Features",
				"UsageType": "HealthCheck-StringMatch",
			},
		},
		{
			Provider: "aws-test",
			SKU:      "TESTPROD-R53-HC-FEAT-LATENCY",
			Service:  "AmazonRoute53",
			Family:   "DNS Health Check",
			Location: "us-east-1",
			Attributes: map[string]string{
				"Group":     "HealthCheck-Features",
				"UsageType": "HealthCheck-LatencyMeasurement",
			},
		},
	}

	for _, p := range prods {
		var err error
		p.ID, err = backend.Products().Upsert(ctx, p)
		require.NoError(t, err)
	}

	prices := []*price.WithProduct{
		{
			Product: prods[0],
			Price: price.Price{
				Unit:     "Hrs",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.12),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[1],
			Price: price.Price{
				Unit:     "Hrs",
				Currency: "USD",
				Value:    decimal.NewFromFloat(1.23),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[2],
			Price: price.Price{
				Unit:     "GB-Mo",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.45),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		// Lambda prices
		{
			Product: prods[3], // Lambda requests
			Price: price.Price{
				Unit:     "Requests",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000002),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[4], // Lambda duration x86
			Price: price.Price{
				Unit:     "seconds",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000166667),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[5], // Lambda duration ARM
			Price: price.Price{
				Unit:     "seconds",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000133334),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		// DynamoDB prices
		{
			Product: prods[6], // DDB on-demand read
			Price: price.Price{
				Unit:     "ReadRequestUnit",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.00000025),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[7], // DDB on-demand write
			Price: price.Price{
				Unit:     "WriteRequestUnit",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.00000125),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[8], // DDB storage
			Price: price.Price{
				Unit:     "GB-Mo",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.25),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[9], // DDB PITR
			Price: price.Price{
				Unit:     "GB-Mo",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.20),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[10], // DDB stream
			Price: price.Price{
				Unit:     "ReadRequestUnit",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.00000002),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[11], // DDB provisioned RCU
			Price: price.Price{
				Unit:     "ReadCapacityUnit-Hrs",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.000139),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[12], // DDB provisioned WCU
			Price: price.Price{
				Unit:     "WriteCapacityUnit-Hrs",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.000694),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		// API Gateway prices
		{
			Product: prods[13], // REST API requests
			Price: price.Price{
				Unit:     "Requests",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000035),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[14], // HTTP API requests (tier 1)
			Price: price.Price{
				Unit:     "Requests",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.000001),
				Attributes: map[string]string{
					"TermType":      "OnDemand",
					"StartingRange": "0",
				},
			},
		},
		{
			Product: prods[15], // WebSocket messages
			Price: price.Price{
				Unit:     "Messages",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.000001),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		// CloudFront prices
		{
			Product: prods[16], // CF data transfer NA
			Price: price.Price{
				Unit:     "GB",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.085),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[17], // CF data transfer EU
			Price: price.Price{
				Unit:     "GB",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.085),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[18], // CF HTTPS requests
			Price: price.Price{
				Unit:     "10k requests",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.01),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[19], // CF invalidation
			Price: price.Price{
				Unit:     "requests",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.005),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[20], // CF function invocations
			Price: price.Price{
				Unit:     "Requests",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000001),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		// Route 53 prices
		{
			Product: prods[21], // R53 hosted zone
			Price: price.Price{
				Unit:     "zones",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.50),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[22], // R53 standard queries
			Price: price.Price{
				Unit:     "Queries",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000004),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[23], // R53 latency queries
			Price: price.Price{
				Unit:     "Queries",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.0000006),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[24], // R53 basic health check
			Price: price.Price{
				Unit:     "HealthCheck",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.50),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[25], // R53 custom health check
			Price: price.Price{
				Unit:     "HealthCheck",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.75),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[26], // R53 health check string matching feature
			Price: price.Price{
				Unit:     "HealthCheck",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.50),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
		{
			Product: prods[27], // R53 health check latency measurement feature
			Price: price.Price{
				Unit:     "HealthCheck",
				Currency: "USD",
				Value:    decimal.NewFromFloat(0.50),
				Attributes: map[string]string{
					"TermType": "OnDemand",
				},
			},
		},
	}

	for _, p := range prices {
		_, err := backend.Prices().Upsert(ctx, p)
		require.NoError(t, err)
	}

	t.Run("TFPlan", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/terraform-plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(91.2), "USD"), pcost)

			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(901.5), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 2)

			for _, diff := range diffs {
				switch diff.Address {
				case "aws_instance.example":
					compute := diff.ComponentDiffs["Compute"]
					require.NotNil(t, compute)
					assert.Equal(t, []string{"Linux", "on-demand", "t2.micro"}, compute.Prior.Details)
					assert.Equal(t, []string{"Linux", "on-demand", "t2.xlarge"}, compute.Planned.Details)
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(87.6), "USD"), compute.PriorCost())
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(897.9), "USD"), compute.PlannedCost())

					rootVol := diff.ComponentDiffs["Root volume: Storage"]
					require.NotNil(t, rootVol)
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(3.6), "USD"), compute.PriorCost())
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(3.6), "USD"), compute.PlannedCost())

					priorCost, err := diff.PriorCost()
					require.NoError(t, err)
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(91.2), "USD"), priorCost)

					plannedCost, err := diff.PlannedCost()
					require.NoError(t, err)
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(901.5), "USD"), plannedCost)

				case "aws_lb.example":
					lb := diff.ComponentDiffs["Application Load Balancer"]
					require.NotNil(t, lb)
					assert.False(t, diff.Valid())
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), lb.Planned.Cost())

					priorCost, err := diff.PriorCost()
					require.NoError(t, err)
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), priorCost)

					plannedCost, err := diff.PlannedCost()
					require.NoError(t, err)
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), plannedCost)
				}
			}
		})
		t.Run("SuccessNoRegion", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/terraform-plan-no-region.json")
			require.NoError(t, err)
			defer f.Close()

			// Instead of using the default one we have a custom one that will
			// basically only work if the region is not set and will set it to
			// the one we are importing by default so we do not have to import
			// other regions for testing
			tfpi := terraform.ProviderInitializer{
				MatchNames: []string{aws.ProviderName, aws.RegistryName},
				Provider: func(values map[string]interface{}) (terraform.Provider, error) {
					_, ok := values["region"]
					if ok {
						return nil, nil
					}
					regCode := region.Code("eu-west-1")
					return awstf.NewProvider(aws.ProviderName, regCode)
				},
			}
			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, tfpi)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assert.Equal(t, cost.Zero, pcost)

			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(31.544), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 1)
			require.Len(t, diffs[0].ComponentDiffs, 2)
		})
		t.Run("SuccessASG", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/asg-plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(665.133), "USD"), pcost)
		})
		t.Run("SuccessEKS", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/eks-plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(99.798), "USD"), pcost)
		})
		t.Run("SuccessLambda", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/lambda/plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			// basic (x86): requests=$0.20, duration=$0.416667 → $0.616667
			// arm_large (arm64): requests=$0.20, duration=$38.9998, ephemeral=$6.6667 → $45.8665
			// Total: ~$46.483
			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(46.483), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 2)
		})
		t.Run("SuccessDynamoDB", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/dynamodb/plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			// on-demand: read=$0.25, write=$0.25, storage=$12.50 → $13.00
			// provisioned: RCU=$1.0147, WCU=$2.5331, storage=$12.50 → $16.0478
			// with_pitr: read=$0.25, write=$0.25, storage=$12.50, PITR=$10.00 → $23.00
			// with_stream: read=$0.25, write=$0.25, storage=$12.50, stream=$0.02 → $13.02
			// Total: ~$65.068
			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(65.068), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 4)
		})
		t.Run("SuccessAPIGateway", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/api_gateway/plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			// rest_api: 5M requests * $0.0000035 = $17.50
			// http_api: 5M requests * $0.000001 = $5.00 (all in first tier)
			// ws_api: 1M messages * $0.000001 = $1.00
			// Total: $23.50
			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(23.50), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 3)
		})
		t.Run("SuccessCloudFront", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/cloudfront/plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			// Distribution: NA 1000 GB * $0.085 = $85.00
			//               EU 200 GB * $0.085 = $17.00
			//               HTTPS 10M / 10K * $0.01 = $10.00
			//               Invalidation: 100 (under 1000 free tier) = $0.00
			// Function: 2M / 1M * $0.10 = $0.20
			// Total: $112.20
			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(112.20), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 2)
		})
		t.Run("SuccessRoute53", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/route53/plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(0), ""), pcost)

			// Zone: $0.50 (hosted zone)
			// Standard queries: 1M * $0.0000004 = $0.40 (default usage)
			// Basic health check: $0.50
			// Custom health check: $0.75 + $0.50 (string match) + $0.50 (latency measurement) = $1.75
			// Records: $0.00 (configuration-only)
			// Total: $0.50 + $0.40 + $0.50 + $1.75 = $3.15
			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(3.15), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 5)
		})
		t.Run("SuccessNoPrior", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/terraform-noprior-plan.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default)
			require.NoError(t, err)

			pcost, err := plan.PriorCost()
			assert.NoError(t, err)
			assert.Equal(t, cost.Zero, pcost)

			pcost, err = plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(31.544), "USD"), pcost)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 1)
			require.Len(t, diffs[0].ComponentDiffs, 2)
		})

		t.Run("ProductNotFound", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/terraform-plan-invalid.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.NoError(t, err)

			diffs := plan.ResourceDifferences()
			require.Len(t, diffs, 1)
			rd := diffs[0]

			rootVol := diffs[0].ComponentDiffs["Root volume: Storage"]
			require.NotNil(t, rootVol)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(3.6), "USD"), rootVol.PriorCost())
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(3.6), "USD"), rootVol.PlannedCost())

			expected := map[string]error{
				"Compute": cost.ErrProductNotFound,
			}
			assert.Equal(t, expected, rd.Errors())
		})
		t.Run("NoProvider", func(t *testing.T) {
			f, err := os.Open("../testdata/aws/terraform-plan-noprovider.json")
			require.NoError(t, err)
			defer f.Close()

			plan, err := costestimation.EstimateTerraformPlan(ctx, backend, f, usage.Default, terraformAWSTestProviderInitializer)
			require.Error(t, err, terraform.ErrNoProviders)
			require.Nil(t, plan)
		})
	})
	t.Run("HCL", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {

			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-aws", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			plan := plans[0]

			assert.Nil(t, plan.Prior)

			pcost, err := plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(62.716), "USD"), pcost)
		})
		t.Run("SuccessMagento", func(t *testing.T) {

			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-magento", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			plan := plans[0]

			assert.Nil(t, plan.Prior)

			pcost, err := plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(86.474), "USD"), pcost)
		})
		t.Run("SuccessASG", func(t *testing.T) {

			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-asg", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			plan := plans[0]

			assert.Nil(t, plan.Prior)

			pcost, err := plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(690.983), "USD"), pcost)
		})
		t.Run("SuccessEKS", func(t *testing.T) {

			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-eks", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			plan := plans[0]

			assert.Nil(t, plan.Prior)

			pcost, err := plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(99.798), "USD"), pcost)
		})
		t.Run("SuccessRemote", func(t *testing.T) {

			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-remote", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 1)
			plan := plans[0]

			assert.Nil(t, plan.Prior)

			pcost, err := plan.PlannedCost()
			assert.NoError(t, err)
			assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(86.474), "USD"), pcost)
		})
		t.Run("SuccessTerragrunt", func(t *testing.T) {
			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/terragrunt/", "../testdata/aws/terragrunt/non-prod/us-east-1/qa/webserver-cluster/", noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 2)

			for i, plan := range plans {
				assert.Nil(t, plan.Prior)
				assert.NotEmpty(t, plan.Name)

				pcost, err := plan.PlannedCost()
				assert.NoError(t, err)
				if i == 0 {
					assert.Equal(t, plan.Name, "mysql")
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(14.41), "USD"), pcost)
				} else {
					assert.Equal(t, plan.Name, "webserver-cluster")
					assertCostEqual(t, cost.NewMonthly(decimal.NewFromFloat(18.25), "USD"), pcost)
				}
			}
		})
		t.Run("TerragruntContextCancelled", func(t *testing.T) {
			ctx, _ = context.WithTimeoutCause(ctx, time.Millisecond, fmt.Errorf("potato"))
			_, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/terragrunt/", "../testdata/aws/terragrunt/non-prod/us-east-1/qa/webserver-cluster/", noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.EqualError(t, err, "potato")

		})
		t.Run("SuccessFunctions", func(t *testing.T) {
			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-functions/", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans, 1)
		})
		t.Run("SuccessCount", func(t *testing.T) {
			//log.Level.Set(slog.LevelDebug)
			plans, err := costestimation.EstimateHCL(ctx, backend, nil, "../testdata/aws/stack-count/", noModulePath, noForceTerragrunt, noParallelismTerragrunt, usage.Default, noDebug)
			require.NoError(t, err)
			require.Len(t, plans[0].Planned.Resources, 12)
		})
	})
}

func assertCostEqual(t *testing.T, expected, actual cost.Cost) {
	assert.Truef(t, expected.Equal(actual.Decimal), "Not equal:\nexpected value: %s\nactual value: %s", expected, actual)
	assert.Truef(t, expected.Currency == actual.Currency, "Not equal:\nexpected currency: %s\nactual currency: %s", expected.Currency, actual.Currency)
}

func JSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}
