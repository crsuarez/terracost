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
	"github.com/cycloidio/terracost/util"
)

func TestRoute53HealthCheck_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("BasicAWS", func(t *testing.T) {
		// Basic HTTP health check targeting AWS endpoint — $0.50/month
		tfres := terraform.Resource{
			Address:      "aws_route53_health_check.basic",
			Type:         "aws_route53_health_check",
			Name:         "basic",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"type":             "HTTP",
				"fqdn":             "example.com",
				"port":             80,
				"request_interval": 30,
			},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name:            "Basic health check (AWS endpoint)",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-AWS-Endpoint")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("CustomWithHTTPS", func(t *testing.T) {
		// Custom HTTPS health check with SNI — $0.75 + $0.50 = $1.25/month
		tfres := terraform.Resource{
			Address:      "aws_route53_health_check.custom_https",
			Type:         "aws_route53_health_check",
			Name:         "custom_https",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"type":             "HTTPS",
				"fqdn":             "api.example.com",
				"port":             443,
				"request_interval": 30,
				"enable_sni":       true,
			},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name:            "Custom health check",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-Custom-Endpoint")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "SNI",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-Features")},
						{Key: "UsageType", Value: util.StringPtr("HealthCheck-SNI")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("CustomWithMultipleFeatures", func(t *testing.T) {
		// Custom health check with string matching + latency measurement + fast interval
		// $0.75 (custom) + $0.50 (string match) + $0.50 (latency) + $1.00 (fast interval) = $2.75/month
		tfres := terraform.Resource{
			Address:      "aws_route53_health_check.full",
			Type:         "aws_route53_health_check",
			Name:         "full",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"type":              "HTTP_STR_MATCH",
				"fqdn":              "health.example.com",
				"port":              80,
				"request_interval":  10,
				"measure_latency":   true,
				"search_string":     "OK",
			},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name:            "Custom health check",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-Custom-Endpoint")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "String matching",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-Features")},
						{Key: "UsageType", Value: util.StringPtr("HealthCheck-StringMatch")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Latency measurement",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-Features")},
						{Key: "UsageType", Value: util.StringPtr("HealthCheck-LatencyMeasurement")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Fast interval",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-Features")},
						{Key: "UsageType", Value: util.StringPtr("HealthCheck-FastInterval")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("TCPBasic", func(t *testing.T) {
		// Basic TCP health check — $0.50/month
		tfres := terraform.Resource{
			Address:      "aws_route53_health_check.tcp",
			Type:         "aws_route53_health_check",
			Name:         "tcp",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"type":             "TCP",
				"fqdn":             "db.example.com",
				"port":             5432,
				"request_interval": 30,
			},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name:            "Basic health check (AWS endpoint)",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Health Check"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HealthCheck-AWS-Endpoint")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("HealthCheck"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
