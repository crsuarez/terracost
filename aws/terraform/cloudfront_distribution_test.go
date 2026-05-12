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

func TestCloudFrontDistribution_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("PriceClass100", func(t *testing.T) {
		// PriceClass_100: NA + EU only
		tfres := terraform.Resource{
			Address:      "aws_cloudfront_distribution.test",
			Type:         "aws_cloudfront_distribution",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"price_class": "PriceClass_100",
			},
		}
		rss := map[string]terraform.Resource{}

		// 1000 GB NA, 200 GB EU, 10M HTTPS requests, 100 invalidations (under free tier)
		expected := []query.Component{
			{
				Name:            "Data transfer out (North America)",
				MonthlyQuantity: decimal.NewFromInt(1000),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("North America"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Data transfer out (Europe)",
				MonthlyQuantity: decimal.NewFromInt(200),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("Europe"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "HTTPS requests",
				MonthlyQuantity: decimal.NewFromInt(1000), // 10M / 10K
				Usage:           true,
				Unit:            "10k requests",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("North America"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTPS-Proxy")},
					},
				},
				PriceFilter: &price.Filter{
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_data_transfer_out_gb": map[string]interface{}{
				"North America": 1000.0,
				"Europe":        200.0,
			},
			"monthly_https_requests":        10000000.0,
			"monthly_http_requests":         0.0,
			"monthly_invalidation_requests": 100.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("PriceClass200", func(t *testing.T) {
		// PriceClass_200: NA + EU + Asia Pacific + Middle East + Africa
		tfres := terraform.Resource{
			Address:      "aws_cloudfront_distribution.test",
			Type:         "aws_cloudfront_distribution",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"price_class": "PriceClass_200",
			},
		}
		rss := map[string]terraform.Resource{}

		// 800 GB NA, 300 GB EU, 100 GB Asia, 10 GB ME, 5 GB AF
		// 5M HTTPS requests, 0 invalidations
		expected := []query.Component{
			{
				Name:            "Data transfer out (North America)",
				MonthlyQuantity: decimal.NewFromInt(800),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("North America"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Data transfer out (Europe)",
				MonthlyQuantity: decimal.NewFromInt(300),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("Europe"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Data transfer out (Asia Pacific)",
				MonthlyQuantity: decimal.NewFromInt(100),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("Asia Pacific"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Data transfer out (Middle East)",
				MonthlyQuantity: decimal.NewFromInt(10),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("Middle East"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Data transfer out (Africa)",
				MonthlyQuantity: decimal.NewFromInt(5),
				Usage:           true,
				Unit:            "GB",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("Africa"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "HTTPS requests",
				MonthlyQuantity: decimal.NewFromInt(500), // 5M / 10K
				Usage:           true,
				Unit:            "10k requests",
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					Location: util.StringPtr("North America"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTPS-Proxy")},
					},
				},
				PriceFilter: &price.Filter{
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := map[string]interface{}{
			"monthly_data_transfer_out_gb": map[string]interface{}{
				"North America": 800.0,
				"Europe":        300.0,
				"Asia Pacific":  100.0,
				"Middle East":   10.0,
				"Africa":        5.0,
			},
			"monthly_https_requests":        5000000.0,
			"monthly_http_requests":         0.0,
			"monthly_invalidation_requests": 0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("PriceClassAll", func(t *testing.T) {
		// PriceClass_All: all regions
		tfres := terraform.Resource{
			Address:      "aws_cloudfront_distribution.test",
			Type:         "aws_cloudfront_distribution",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"price_class": "PriceClass_All",
			},
		}
		rss := map[string]terraform.Resource{}

		// 500 GB NA, 200 GB EU, 50 GB Asia, 10 GB SA, 5 GB AU, 20 GB IN, 3 GB ME, 2 GB AF, 8 GB JP
		// 10M HTTPS, 1M HTTP
		expected := []query.Component{
			{
				Name: "Data transfer out (North America)", MonthlyQuantity: decimal.NewFromInt(500),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (Europe)", MonthlyQuantity: decimal.NewFromInt(200),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("Europe"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (Asia Pacific)", MonthlyQuantity: decimal.NewFromInt(50),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("Asia Pacific"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (South America)", MonthlyQuantity: decimal.NewFromInt(10),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("South America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (Australia)", MonthlyQuantity: decimal.NewFromInt(5),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("Australia"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (India)", MonthlyQuantity: decimal.NewFromInt(20),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("India"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (Middle East)", MonthlyQuantity: decimal.NewFromInt(3),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("Middle East"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (Africa)", MonthlyQuantity: decimal.NewFromInt(2),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("Africa"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Data transfer out (Japan)", MonthlyQuantity: decimal.NewFromInt(8),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("Japan"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "HTTPS requests", MonthlyQuantity: decimal.NewFromInt(1000),
				Usage: true, Unit: "10k requests",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTPS-Proxy")}}},
				PriceFilter:   &price.Filter{AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "HTTP requests", MonthlyQuantity: decimal.NewFromInt(100),
				Usage: true, Unit: "10k requests",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTP-Proxy")}}},
				PriceFilter:   &price.Filter{AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
		}

		us := map[string]interface{}{
			"monthly_data_transfer_out_gb": map[string]interface{}{
				"North America": 500.0,
				"Europe":        200.0,
				"Asia Pacific":  50.0,
				"South America": 10.0,
				"Australia":     5.0,
				"India":         20.0,
				"Middle East":   3.0,
				"Africa":        2.0,
				"Japan":         8.0,
			},
			"monthly_https_requests":        10000000.0,
			"monthly_http_requests":         1000000.0,
			"monthly_invalidation_requests": 0.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("InvalidationsBelowFreeTier", func(t *testing.T) {
		// 500 invalidations — below 1000 free tier, no invalidation component
		tfres := terraform.Resource{
			Address:      "aws_cloudfront_distribution.test",
			Type:         "aws_cloudfront_distribution",
			Name:         "test",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name: "Data transfer out (North America)", MonthlyQuantity: decimal.NewFromInt(100),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "HTTPS requests", MonthlyQuantity: decimal.NewFromInt(1000),
				Usage: true, Unit: "10k requests",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTPS-Proxy")}}},
				PriceFilter:   &price.Filter{AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
		}

		us := map[string]interface{}{
			"monthly_data_transfer_out_gb": map[string]interface{}{
				"North America": 100.0,
			},
			"monthly_https_requests":        10000000.0,
			"monthly_http_requests":         0.0,
			"monthly_invalidation_requests": 500.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("InvalidationsAboveFreeTier", func(t *testing.T) {
		// 5000 invalidations — 4000 paid (5000 - 1000 free)
		tfres := terraform.Resource{
			Address:      "aws_cloudfront_distribution.test",
			Type:         "aws_cloudfront_distribution",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"price_class": "PriceClass_100",
			},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name: "Data transfer out (North America)", MonthlyQuantity: decimal.NewFromInt(500),
				Usage: true, Unit: "GB",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-DataTransfer-Out-Bytes")}}},
				PriceFilter:   &price.Filter{Unit: util.StringPtr("GB"), AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "HTTPS requests", MonthlyQuantity: decimal.NewFromInt(1000),
				Usage: true, Unit: "10k requests",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), Location: util.StringPtr("North America"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTPS-Proxy")}}},
				PriceFilter:   &price.Filter{AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
			{
				Name: "Invalidation requests", MonthlyQuantity: decimal.NewFromInt(4000),
				Usage: true, Unit: "requests",
				ProductFilter: &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront"), AttributeFilters: []*product.AttributeFilter{{Key: "Group", Value: util.StringPtr("CloudFront-Invalidation")}}},
				PriceFilter:   &price.Filter{AttributeFilters: []*price.AttributeFilter{{Key: "TermType", Value: util.StringPtr("OnDemand")}}},
			},
		}

		us := map[string]interface{}{
			"monthly_data_transfer_out_gb": map[string]interface{}{
				"North America": 500.0,
			},
			"monthly_https_requests":        10000000.0,
			"monthly_http_requests":         0.0,
			"monthly_invalidation_requests": 5000.0,
		}
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
