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

func TestAPIGatewayRestAPI_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("Minimum", func(t *testing.T) {
		// Minimum config: 5M requests, no cache
		tfres := terraform.Resource{
			Address:      "aws_api_gateway_rest_api.test",
			Type:         "aws_api_gateway_rest_api",
			Name:         "test",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// Requests use the raw AWS PricePerUnit quantity.
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(5000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayRequest")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests": 5000000.0,
			"cache_enabled":    false,
			"cache_size_gb":    0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("WithCache", func(t *testing.T) {
		// REST API with cache enabled (6.1 GB)
		tfres := terraform.Resource{
			Address:      "aws_api_gateway_rest_api.test",
			Type:         "aws_api_gateway_rest_api",
			Name:         "test",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// Requests use the raw AWS PricePerUnit quantity.
		// Cache: 6.1 GB, 744 hours/month
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(10000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayRequest")},
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
				Name:           "Cache memory 6.1 GB",
				HourlyQuantity: decimal.NewFromInt(744),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayCacheUsage")},
						{Key: "UsageType", Value: util.StringPtr("ApiGatewayCacheUsage")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
						{Key: "StartingRange", Value: util.StringPtr("6.1")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests": 10000000.0,
			"cache_enabled":    true,
			"cache_size_gb":    6.1,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("HighVolume", func(t *testing.T) {
		// REST API with high volume (500M requests) — REST does not have tiered pricing
		tfres := terraform.Resource{
			Address:      "aws_api_gateway_rest_api.test",
			Type:         "aws_api_gateway_rest_api",
			Name:         "test",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// Requests use the raw AWS PricePerUnit quantity.
		expected := []query.Component{
			{
				Name:            "Requests",
				MonthlyQuantity: decimal.NewFromInt(500000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayRequest")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests": 500000000.0,
			"cache_enabled":    false,
			"cache_size_gb":    0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
