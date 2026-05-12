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

func TestCloudFrontFunction_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("Basic", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_cloudfront_function.test",
			Type:         "aws_cloudfront_function",
			Name:         "test",
			ProviderName: "aws",
			Values:       map[string]interface{}{},
		}
		rss := map[string]terraform.Resource{}

		// 2M invocations / 1M = 2
		expected := []query.Component{
			{
				Name:            "Invocations",
				MonthlyQuantity: decimal.NewFromInt(2),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonCloudFront"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("CloudFront-EdgeFunctions")},
					},
				},
				PriceFilter: &price.Filter{
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := usage.Default.GetUsage("aws_cloudfront_function")
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
