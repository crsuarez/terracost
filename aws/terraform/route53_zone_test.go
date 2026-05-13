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

func TestRoute53Zone_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("BasicZone", func(t *testing.T) {
		// Vanilla zone with default usage: 1M standard queries
		tfres := terraform.Resource{
			Address:      "aws_route53_zone.main",
			Type:         "aws_route53_zone",
			Name:         "main",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// Default usage: monthly_standard_queries=1_000_000
		// Hosted zone: 1 zone-month = $0.50
		// Queries use the raw AWS PricePerUnit quantity.
		expected := []query.Component{
			{
				Name:            "Hosted zone",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Zone"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HostedZone")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("zones"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Standard queries",
				MonthlyQuantity: decimal.NewFromInt(1000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Query"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DNS-Queries")},
						{Key: "UsageType", Value: util.StringPtr("DNS-Queries")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Queries"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := usage.Default.GetUsage("aws_route53_zone")
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("ZoneWithLatencyQueries", func(t *testing.T) {
		// Zone with latency-based queries
		tfres := terraform.Resource{
			Address:      "aws_route53_zone.latency",
			Type:         "aws_route53_zone",
			Name:         "latency",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// 5M standard queries + 2M latency queries.
		expected := []query.Component{
			{
				Name:            "Hosted zone",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Zone"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HostedZone")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("zones"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Standard queries",
				MonthlyQuantity: decimal.NewFromInt(5000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Query"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DNS-Queries")},
						{Key: "UsageType", Value: util.StringPtr("DNS-Queries")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Queries"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Latency-based queries",
				MonthlyQuantity: decimal.NewFromInt(2000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Query"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DNS-Queries")},
						{Key: "UsageType", Value: util.StringPtr("DNS-LatencyBasedRoutingQueries")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Queries"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_standard_queries": 5000000.0,
			"monthly_latency_queries":  2000000.0,
			"monthly_geo_queries":      0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("ZoneWithGeoQueries", func(t *testing.T) {
		// Zone with geo queries only
		tfres := terraform.Resource{
			Address:      "aws_route53_zone.geo",
			Type:         "aws_route53_zone",
			Name:         "geo",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// 3M geo queries.
		expected := []query.Component{
			{
				Name:            "Hosted zone",
				MonthlyQuantity: decimal.NewFromInt(1),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Zone"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("HostedZone")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("zones"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Geo queries",
				MonthlyQuantity: decimal.NewFromInt(3000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonRoute53"),
					Family:   util.StringPtr("DNS Query"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DNS-Queries")},
						{Key: "UsageType", Value: util.StringPtr("DNS-GeoBasedRoutingQueries")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("Queries"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_standard_queries": 0.0,
			"monthly_latency_queries":  0.0,
			"monthly_geo_queries":      3000000.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
