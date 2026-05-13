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

// cacheSizeToGB maps AWS API Gateway cache cluster sizes (in GB) to their
// string representation used in pricing. Valid values: 0.5, 1.6, 6.1, 13.5,
// 28.4, 58.2, 118, 237.
var cacheSizeToGB = map[int64]string{
	1: "0.5",
	2: "1.6",
	3: "6.1",
	4: "13.5",
	5: "28.4",
	6: "58.2",
	7: "118",
	8: "237",
}

// hoursPerMonth is the number of hours in a 31-day month.
var hoursPerMonth = decimal.NewFromInt(744)

// APIGatewayRestAPI represents an aws_api_gateway_rest_api for cost estimation.
type APIGatewayRestAPI struct {
	provider *Provider
	region   region.Code

	// Usage (from tc_usage)
	monthlyRequests decimal.Decimal

	// Cache (discovered from associated aws_api_gateway_stage)
	cacheEnabled bool
	cacheSizeGB  string // e.g. "0.5", "6.1"
}

type apiGatewayRestAPIValues struct {
	Usage struct {
		MonthlyRequests float64 `mapstructure:"monthly_requests"`
		CacheEnabled    bool    `mapstructure:"cache_enabled"`
		CacheSizeGB     float64 `mapstructure:"cache_size_gb"`
	} `mapstructure:"tc_usage"`
}

// apiGatewayStageValues is used to decode aws_api_gateway_stage resources
// for cross-resource cache lookup.
type apiGatewayStageValues struct {
	RestAPIID        string `mapstructure:"rest_api_id"`
	CacheClusterSize int64  `mapstructure:"cache_cluster_size"`
}

// decodeAPIGatewayRestAPIValues decodes and returns apiGatewayRestAPIValues from a Terraform values map.
func decodeAPIGatewayRestAPIValues(tfVals map[string]interface{}) (apiGatewayRestAPIValues, error) {
	var v apiGatewayRestAPIValues
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

// decodeAPIGatewayStageValues decodes and returns apiGatewayStageValues from a Terraform values map.
func decodeAPIGatewayStageValues(tfVals map[string]interface{}) (apiGatewayStageValues, error) {
	var v apiGatewayStageValues
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

// newAPIGatewayRestAPI creates a new APIGatewayRestAPI from apiGatewayRestAPIValues.
// It walks sibling resources to discover cache configuration from aws_api_gateway_stage.
func (p *Provider) newAPIGatewayRestAPI(rss map[string]terraform.Resource, vals apiGatewayRestAPIValues, tfRes terraform.Resource) *APIGatewayRestAPI {
	api := &APIGatewayRestAPI{
		provider:        p,
		region:          p.region,
		monthlyRequests: decimal.NewFromFloat(vals.Usage.MonthlyRequests),
		cacheEnabled:    vals.Usage.CacheEnabled,
	}

	// Discover cache configuration from tc_usage cache_size_gb
	if vals.Usage.CacheSizeGB > 0 {
		api.cacheSizeGB = formatCacheSize(vals.Usage.CacheSizeGB)
	}

	// Walk sibling resources to find an associated aws_api_gateway_stage
	// with caching enabled. The stage references the REST API by its ID or name.
	for _, res := range rss {
		if res.Type != "aws_api_gateway_stage" {
			continue
		}
		stageVals, err := decodeAPIGatewayStageValues(res.Values)
		if err != nil {
			continue
		}
		// Match stage to this REST API by address reference or name
		if stageVals.RestAPIID == tfRes.Address || stageVals.RestAPIID == tfRes.Name {
			if stageVals.CacheClusterSize > 0 {
				api.cacheEnabled = true
				if gb, ok := cacheSizeToGB[stageVals.CacheClusterSize]; ok {
					api.cacheSizeGB = gb
				}
			}
			break
		}
	}

	return api
}

// formatCacheSize converts a float64 cache size to the nearest valid AWS cache size string.
func formatCacheSize(size float64) string {
	s := decimal.NewFromFloat(size)
	validSizes := []string{"0.5", "1.6", "6.1", "13.5", "28.4", "58.2", "118", "237"}
	bestIdx := 0
	bestDiff := decimal.NewFromInt(999999)
	for i, vs := range validSizes {
		vd, _ := decimal.NewFromString(vs)
		diff := s.Sub(vd).Abs()
		if diff.LessThan(bestDiff) {
			bestDiff = diff
			bestIdx = i
		}
	}
	return validSizes[bestIdx]
}

// Components returns the price component queries that make up this APIGatewayRestAPI.
func (api *APIGatewayRestAPI) Components() []query.Component {
	components := []query.Component{
		api.requestsComponent(),
	}

	if api.cacheEnabled && api.cacheSizeGB != "" {
		components = append(components, api.cacheComponent())
	}

	return components
}

func (api *APIGatewayRestAPI) requestsComponent() query.Component {
	return query.Component{
		Name:            "Requests",
		MonthlyQuantity: api.monthlyRequests,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(api.provider.key),
			Service:  util.StringPtr("AmazonApiGateway"),
			Location: util.StringPtr(api.region.String()),
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
	}
}

func (api *APIGatewayRestAPI) cacheComponent() query.Component {
	return query.Component{
		Name:           "Cache memory " + api.cacheSizeGB + " GB",
		HourlyQuantity: hoursPerMonth,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(api.provider.key),
			Service:  util.StringPtr("AmazonApiGateway"),
			Location: util.StringPtr(api.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("ApiGatewayCacheUsage")},
				{Key: "UsageType", Value: util.StringPtr("ApiGatewayCacheUsage")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("Hrs"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
				{Key: "StartingRange", Value: util.StringPtr(api.cacheSizeGB)},
			},
		},
	}
}
