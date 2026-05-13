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

func TestApigatewayv2API_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("HTTPMinimum", func(t *testing.T) {
		// HTTP API with 100M requests — all in first tier
		tfres := terraform.Resource{
			Address:      "aws_apigatewayv2_api.test",
			Type:         "aws_apigatewayv2_api",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"protocol_type": "HTTP",
			},
		}
		rss := map[string]terraform.Resource{}

		// Requests use raw AWS PricePerUnit quantities.
		expected := []query.Component{
			{
				Name:            "HTTP API requests",
				MonthlyQuantity: decimal.NewFromInt(100000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayHttpRequest")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
						{Key: "StartingRange", Value: util.StringPtr("0")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests":           100000000.0,
			"monthly_message_count":      0.0,
			"monthly_connection_minutes": 0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("HTTPTiered", func(t *testing.T) {
		// HTTP API with 400M requests — crosses the 300M tier boundary
		tfres := terraform.Resource{
			Address:      "aws_apigatewayv2_api.test",
			Type:         "aws_apigatewayv2_api",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"protocol_type": "HTTP",
			},
		}
		rss := map[string]terraform.Resource{}

		// First 300M at tier 0, remaining 100M at tier 1.
		expected := []query.Component{
			{
				Name:            "HTTP API requests",
				MonthlyQuantity: decimal.NewFromInt(300000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayHttpRequest")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
						{Key: "StartingRange", Value: util.StringPtr("0")},
					},
				},
			},
			{
				Name:            "HTTP API requests",
				MonthlyQuantity: decimal.NewFromInt(100000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayHttpRequest")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Requests"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
						{Key: "StartingRange", Value: util.StringPtr("300000000")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests":           400000000.0,
			"monthly_message_count":      0.0,
			"monthly_connection_minutes": 0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("WebSocket", func(t *testing.T) {
		// WebSocket API with 1M messages and 10K connection minutes
		tfres := terraform.Resource{
			Address:      "aws_apigatewayv2_api.test",
			Type:         "aws_apigatewayv2_api",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"protocol_type": "WEBSOCKET",
			},
		}
		rss := map[string]terraform.Resource{}

		// Messages use raw AWS PricePerUnit quantities.
		// Connection minutes: 10000
		expected := []query.Component{
			{
				Name:            "WebSocket messages",
				MonthlyQuantity: decimal.NewFromInt(1000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayWebSocketMessage")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Messages"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "WebSocket connection minutes",
				MonthlyQuantity: decimal.NewFromInt(10000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonApiGateway"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("ApiGatewayWebSocketMessage")},
						{Key: "UsageType", Value: util.StringPtr("ApiGatewayWebSocket-ConnMinutes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("minutes"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_requests":           0.0,
			"monthly_message_count":      1000000.0,
			"monthly_connection_minutes": 10000.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
