package terraform

import (
	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"

	"github.com/cycloidio/terracost/aws/region"
	"github.com/cycloidio/terracost/price"
	"github.com/cycloidio/terracost/product"
	"github.com/cycloidio/terracost/query"
	"github.com/cycloidio/terracost/terraform"
	"github.com/cycloidio/terracost/util"
)

// CloudFrontFunction represents an aws_cloudfront_function for cost estimation.
type CloudFrontFunction struct {
	provider *Provider
	region   region.Code

	// Usage (from tc_usage)
	monthlyInvocations decimal.Decimal
}

type cloudFrontFunctionValues struct {
	Usage struct {
		MonthlyInvocations float64 `mapstructure:"monthly_invocations"`
	} `mapstructure:"tc_usage"`
}

// decodeCloudFrontFunctionValues decodes and returns cloudFrontFunctionValues from a Terraform values map.
func decodeCloudFrontFunctionValues(tfVals map[string]interface{}) (cloudFrontFunctionValues, error) {
	var v cloudFrontFunctionValues
	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           &v,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return v, err
	}

	if err := decoder.Decode(tfVals); err != nil {
		return v, err
	}
	return v, nil
}

// newCloudFrontFunction creates a new CloudFrontFunction from cloudFrontFunctionValues.
func (p *Provider) newCloudFrontFunction(_ map[string]terraform.Resource, vals cloudFrontFunctionValues) *CloudFrontFunction {
	return &CloudFrontFunction{
		provider:           p,
		region:             p.region,
		monthlyInvocations: decimal.NewFromFloat(vals.Usage.MonthlyInvocations),
	}
}

// Components returns the price component queries that make up this CloudFrontFunction.
func (cf *CloudFrontFunction) Components() []query.Component {
	return []query.Component{
		cf.invocationsComponent(),
	}
}

// invocationsComponent returns the CloudFront Functions invocations component.
func (cf *CloudFrontFunction) invocationsComponent() query.Component {
	return query.Component{
		Name:            "Invocations",
		MonthlyQuantity: cf.monthlyInvocations,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(cf.provider.key),
			Service:  util.StringPtr("AmazonCloudFront"),
			Family:   util.StringPtr("CloudFront Functions"),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("CloudFront-EdgeFunctions")},
				{Key: "UsageType", Value: util.StringPtr("CloudFront-Function-Invocations")},
			},
		},
		PriceFilter: &price.Filter{
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}
