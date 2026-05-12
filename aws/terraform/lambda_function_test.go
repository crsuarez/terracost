package terraform_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	awstf "github.com/cycloidio/terracost/aws/terraform"
	"github.com/cycloidio/terracost/price"
	"github.com/cycloidio/terracost/product"
	"github.com/cycloidio/terracost/query"
	"github.com/cycloidio/terracost/terraform"
	"github.com/cycloidio/terracost/testutil"
	"github.com/cycloidio/terracost/usage"
	"github.com/cycloidio/terracost/util"
)

func TestLambdaFunction_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("Defaults", func(t *testing.T) {
		// Minimum config: 128 MB, x86_64, defaults
		tfres := terraform.Resource{
			Address:      "aws_lambda_function.test",
			Type:         "aws_lambda_function",
			Name:         "test",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// Default usage: monthly_requests=1_000_000, request_duration_ms=200
		// Requests: 1_000_000 / 1_000_000 = 1
		// Duration GB-seconds: (128/1024) * 1_000_000 * (200/1000) = 25_000
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(1),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Requests")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Duration",
				MonthlyQuantity: decimal.NewFromInt(25000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Duration")},
						{Key: "UsageType", ValueRegex: util.StringPtr(".*Lambda-GB-Seconds")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := usage.Default.GetUsage("aws_lambda_function")
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("LargeMemoryARM", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_lambda_function.test",
			Type:         "aws_lambda_function",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"architectures": []string{"arm64"},
				"memory_size":   3008,
				"ephemeral_storage": []map[string]interface{}{
					{"size": 1024},
				},
			},
		}
		rss := map[string]terraform.Resource{}

		// 2M requests, 500ms duration
		// Requests: 2_000_000 / 1_000_000 = 2
		// Duration GB-seconds: (3008/1024) * 2_000_000 * (500/1000) = 2_937_500
		// Ephemeral: 1024 MB - 512 MB = 512 MB = 0.5 GB
		//   GB-seconds: 0.5 * 2_000_000 * 0.5 = 500_000
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(2),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Requests")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Duration",
				MonthlyQuantity: decimal.NewFromInt(2937500),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Duration")},
						{Key: "UsageType", ValueRegex: util.StringPtr(".*Lambda-ARM-GB-Seconds")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Ephemeral storage",
				MonthlyQuantity: decimal.NewFromInt(500000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Duration")},
						{Key: "UsageType", ValueRegex: util.StringPtr(".*Lambda-ARM-GB-Seconds")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests":                     2000000.0,
			"request_duration_ms":                  500.0,
			"monthly_provisioned_concurrency_seconds": 0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("EphemeralStorageAboveDefault", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_lambda_function.test",
			Type:         "aws_lambda_function",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"memory_size": 256,
				"ephemeral_storage": []map[string]interface{}{
					{"size": 2048},
				},
			},
		}
		rss := map[string]terraform.Resource{}

		// 5M requests, 100ms duration
		// Requests: 5_000_000 / 1_000_000 = 5
		// Duration GB-seconds: (256/1024) * 5_000_000 * (100/1000) = 125_000
		// Ephemeral: 2048 - 512 = 1536 MB = 1.5 GB
		//   GB-seconds: 1.5 * 5_000_000 * 0.1 = 750_000
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(5),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Requests")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Duration",
				MonthlyQuantity: decimal.NewFromInt(125000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Duration")},
						{Key: "UsageType", ValueRegex: util.StringPtr(".*Lambda-GB-Seconds")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Ephemeral storage",
				MonthlyQuantity: decimal.NewFromInt(750000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Duration")},
						{Key: "UsageType", ValueRegex: util.StringPtr(".*Lambda-GB-Seconds")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests":                     5000000.0,
			"request_duration_ms":                  100.0,
			"monthly_provisioned_concurrency_seconds": 0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("ProvisionedConcurrency", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_lambda_function.test",
			Type:         "aws_lambda_function",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"memory_size": 512,
			},
		}
		rss := map[string]terraform.Resource{}

		// 1M requests, 300ms duration, 10M provisioned concurrency seconds
		// Requests: 1_000_000 / 1_000_000 = 1
		// Duration GB-seconds: (512/1024) * 1_000_000 * (300/1000) = 150_000
		// Provisioned concurrency GB-seconds: (512/1024) * 10_000_000 = 5_000_000
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(1),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Requests")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Duration",
				MonthlyQuantity: decimal.NewFromInt(150000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Duration")},
						{Key: "UsageType", ValueRegex: util.StringPtr(".*Lambda-GB-Seconds")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Provisioned concurrency",
				MonthlyQuantity: decimal.NewFromInt(5000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AWSLambda"),
					Family:   util.StringPtr("Serverless"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("AWS-Lambda-Provisioned")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("seconds"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests":                     1000000.0,
			"request_duration_ms":                  300.0,
			"monthly_provisioned_concurrency_seconds": 10000000.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
