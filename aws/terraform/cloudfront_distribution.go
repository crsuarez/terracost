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

var (
	// requestsPerUnit is the divisor for CloudFront request pricing (per 10,000 requests).
	requestsPerUnit = decimal.NewFromInt(10_000)

	// invalidationFreeTier is the number of free invalidation paths per month.
	invalidationFreeTier = decimal.NewFromInt(1000)
)

// cloudFrontRegionGroup represents a CloudFront edge region group with its
// pricing Location string and UsageType pattern for data transfer.
type cloudFrontRegionGroup struct {
	name      string
	location  string
	usageType string
}

// cloudFrontRegionGroups defines all CloudFront edge region groups.
// The location values match the "Location" column in the AWS CloudFront bulk pricing CSV.
var cloudFrontRegionGroups = []cloudFrontRegionGroup{
	{name: "North America", location: "North America", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "Europe", location: "Europe", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "Asia Pacific", location: "Asia Pacific", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "South America", location: "South America", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "Australia", location: "Australia", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "India", location: "India", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "Middle East", location: "Middle East", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "Africa", location: "Africa", usageType: "CloudFront-DataTransfer-Out-Bytes"},
	{name: "Japan", location: "Japan", usageType: "CloudFront-DataTransfer-Out-Bytes"},
}

// cloudFrontPriceClassRegions maps each CloudFront price class to the
// region groups it includes. PriceClass_100 = NA+EU, PriceClass_200 =
// +Asia+ME+AF, PriceClass_All = all edge locations.
var cloudFrontPriceClassRegions = map[string][]string{
	"PriceClass_100": {"North America", "Europe"},
	"PriceClass_200": {"North America", "Europe", "Asia Pacific", "Middle East", "Africa"},
	"PriceClass_All": {"North America", "Europe", "Asia Pacific", "South America", "Australia", "India", "Middle East", "Africa", "Japan"},
}

// CloudFrontDistribution represents an aws_cloudfront_distribution for cost estimation.
type CloudFrontDistribution struct {
	provider *Provider
	region   region.Code

	priceClass string

	// Usage (from tc_usage)
	monthlyDataTransferOutGB map[string]decimal.Decimal // keyed by region group name
	monthlyHTTPSRequests     decimal.Decimal
	monthlyHTTPRequests      decimal.Decimal
	monthlyInvalidationRequests decimal.Decimal
}

type cloudFrontDistributionValues struct {
	PriceClass string `mapstructure:"price_class"`

	Usage struct {
		MonthlyDataTransferOutGB  map[string]float64 `mapstructure:"monthly_data_transfer_out_gb"`
		MonthlyHTTPSRequests      float64            `mapstructure:"monthly_https_requests"`
		MonthlyHTTPRequests       float64            `mapstructure:"monthly_http_requests"`
		MonthlyInvalidationRequests float64          `mapstructure:"monthly_invalidation_requests"`
	} `mapstructure:"tc_usage"`
}

// decodeCloudFrontDistributionValues decodes and returns cloudFrontDistributionValues from a Terraform values map.
func decodeCloudFrontDistributionValues(tfVals map[string]interface{}) (cloudFrontDistributionValues, error) {
	var v cloudFrontDistributionValues
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

// newCloudFrontDistribution creates a new CloudFrontDistribution from cloudFrontDistributionValues.
func (p *Provider) newCloudFrontDistribution(_ map[string]terraform.Resource, vals cloudFrontDistributionValues) *CloudFrontDistribution {
	cf := &CloudFrontDistribution{
		provider:    p,
		region:      p.region,
		priceClass:  "PriceClass_All", // AWS default
		monthlyHTTPSRequests:        decimal.NewFromFloat(vals.Usage.MonthlyHTTPSRequests),
		monthlyHTTPRequests:         decimal.NewFromFloat(vals.Usage.MonthlyHTTPRequests),
		monthlyInvalidationRequests: decimal.NewFromFloat(vals.Usage.MonthlyInvalidationRequests),
	}

	if vals.PriceClass != "" {
		cf.priceClass = vals.PriceClass
	}

	// Parse per-region-group data transfer usage
	cf.monthlyDataTransferOutGB = make(map[string]decimal.Decimal)
	for _, rg := range cloudFrontRegionGroups {
		if gb, ok := vals.Usage.MonthlyDataTransferOutGB[rg.name]; ok {
			cf.monthlyDataTransferOutGB[rg.name] = decimal.NewFromFloat(gb)
		}
	}

	return cf
}

// Components returns the price component queries that make up this CloudFrontDistribution.
func (cf *CloudFrontDistribution) Components() []query.Component {
	var components []query.Component

	// Data transfer out per region group (filtered by price class)
	components = append(components, cf.dataTransferComponents()...)

	// HTTPS requests
	if cf.monthlyHTTPSRequests.GreaterThan(decimal.Zero) {
		components = append(components, cf.httpsRequestsComponent())
	}

	// HTTP requests
	if cf.monthlyHTTPRequests.GreaterThan(decimal.Zero) {
		components = append(components, cf.httpRequestsComponent())
	}

	// Invalidation requests (first 1000 free)
	if cf.monthlyInvalidationRequests.GreaterThan(invalidationFreeTier) {
		components = append(components, cf.invalidationComponent())
	}

	return components
}

// activeRegionGroups returns the region groups included in the distribution's price class.
func (cf *CloudFrontDistribution) activeRegionGroups() []cloudFrontRegionGroup {
	activeNames, ok := cloudFrontPriceClassRegions[cf.priceClass]
	if !ok {
		// Default to all regions if price class is unknown
		activeNames = cloudFrontPriceClassRegions["PriceClass_All"]
	}

	activeSet := make(map[string]bool, len(activeNames))
	for _, name := range activeNames {
		activeSet[name] = true
	}

	var result []cloudFrontRegionGroup
	for _, rg := range cloudFrontRegionGroups {
		if activeSet[rg.name] {
			result = append(result, rg)
		}
	}
	return result
}

// dataTransferComponents returns one data transfer component per active region group.
func (cf *CloudFrontDistribution) dataTransferComponents() []query.Component {
	var components []query.Component
	for _, rg := range cf.activeRegionGroups() {
		gb := cf.monthlyDataTransferOutGB[rg.name]
		if gb.LessThanOrEqual(decimal.Zero) {
			continue
		}
		components = append(components, query.Component{
			Name:            "Data transfer out (" + rg.name + ")",
			MonthlyQuantity: gb,
			Usage:           true,
			Unit:            "GB",
			ProductFilter: &product.Filter{
				Provider: util.StringPtr(cf.provider.key),
				Service:  util.StringPtr("AmazonCloudFront"),
				Location: util.StringPtr(rg.location),
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
		})
	}
	return components
}

// httpsRequestsComponent returns the HTTPS requests component.
func (cf *CloudFrontDistribution) httpsRequestsComponent() query.Component {
	return query.Component{
		Name:            "HTTPS requests",
		MonthlyQuantity: cf.monthlyHTTPSRequests.Div(requestsPerUnit),
		Usage:           true,
		Unit:            "10k requests",
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(cf.provider.key),
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
	}
}

// httpRequestsComponent returns the HTTP requests component.
func (cf *CloudFrontDistribution) httpRequestsComponent() query.Component {
	return query.Component{
		Name:            "HTTP requests",
		MonthlyQuantity: cf.monthlyHTTPRequests.Div(requestsPerUnit),
		Usage:           true,
		Unit:            "10k requests",
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(cf.provider.key),
			Service:  util.StringPtr("AmazonCloudFront"),
			Location: util.StringPtr("North America"),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("CloudFront-Requests-HTTP-Proxy")},
			},
		},
		PriceFilter: &price.Filter{
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// invalidationComponent returns the invalidation requests component.
// Only emitted when usage exceeds the 1000/month free tier.
func (cf *CloudFrontDistribution) invalidationComponent() query.Component {
	paidInvalidations := cf.monthlyInvalidationRequests.Sub(invalidationFreeTier)
	return query.Component{
		Name:            "Invalidation requests",
		MonthlyQuantity: paidInvalidations,
		Usage:           true,
		Unit:            "requests",
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(cf.provider.key),
			Service:  util.StringPtr("AmazonCloudFront"),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("CloudFront-Invalidation")},
			},
		},
		PriceFilter: &price.Filter{
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}
