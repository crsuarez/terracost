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

// Route53Zone represents an aws_route53_zone for cost estimation.
//
// Pricing dimensions (per AWS Route 53 pricing page):
//   - Hosted zone: $0.50 per zone per month (first 25 zones; $0.10 for zones 26+).
//     NOTE: The tier-1 vs tier-2 boundary is an account-level concept (total zone count
//     across the account). Terracost models individual resources, so we always emit the
//     tier-1 SKU ($0.50/zone-month). The actual cost may be lower for accounts with >25 zones.
//   - Standard DNS queries: $0.40 per million queries (first 1B), $0.20 per million (1B+).
//   - Latency-based / geoproximity queries: $0.60 per million queries (first 1B), $0.30/M (1B+).
//
// The zone resource owns the DNS query cost via tc_usage. Record resources (aws_route53_record)
// are configuration-only and do not emit cost components; the routing policy that determines
// query pricing tier is declared on the zone's tc_usage.
type Route53Zone struct {
	provider *Provider
	region   region.Code

	// Usage (from tc_usage)
	monthlyStandardQueries decimal.Decimal
	monthlyLatencyQueries  decimal.Decimal
	monthlyGeoQueries      decimal.Decimal
}

type route53ZoneValues struct {
	// No resource-level fields influence cost; all pricing is via tc_usage.
	Usage struct {
		MonthlyStandardQueries float64 `mapstructure:"monthly_standard_queries"`
		MonthlyLatencyQueries  float64 `mapstructure:"monthly_latency_queries"`
		MonthlyGeoQueries      float64 `mapstructure:"monthly_geo_queries"`
	} `mapstructure:"tc_usage"`
}

// decodeRoute53ZoneValues decodes and returns route53ZoneValues from a Terraform values map.
func decodeRoute53ZoneValues(tfVals map[string]interface{}) (route53ZoneValues, error) {
	var v route53ZoneValues
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

// newRoute53Zone creates a new Route53Zone from route53ZoneValues.
func (p *Provider) newRoute53Zone(_ map[string]terraform.Resource, vals route53ZoneValues) *Route53Zone {
	return &Route53Zone{
		provider: p,
		region:   p.region,

		monthlyStandardQueries: decimal.NewFromFloat(vals.Usage.MonthlyStandardQueries),
		monthlyLatencyQueries:  decimal.NewFromFloat(vals.Usage.MonthlyLatencyQueries),
		monthlyGeoQueries:      decimal.NewFromFloat(vals.Usage.MonthlyGeoQueries),
	}
}

// Components returns the price component queries that make up this Route53Zone.
func (z *Route53Zone) Components() []query.Component {
	components := []query.Component{
		z.hostedZoneComponent(),
	}

	if z.monthlyStandardQueries.GreaterThan(decimal.Zero) {
		components = append(components, z.standardQueriesComponent())
	}

	if z.monthlyLatencyQueries.GreaterThan(decimal.Zero) {
		components = append(components, z.latencyQueriesComponent())
	}

	if z.monthlyGeoQueries.GreaterThan(decimal.Zero) {
		components = append(components, z.geoQueriesComponent())
	}

	return components
}

// hostedZoneComponent returns the per-zone-month hosted zone component.
// Always uses the tier-1 SKU ($0.50/zone-month). See struct comment for tier-2 limitation.
func (z *Route53Zone) hostedZoneComponent() query.Component {
	return query.Component{
		Name:            "Hosted zone",
		MonthlyQuantity: decimal.NewFromInt(1),
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(z.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Zone"),
			Location: util.StringPtr(z.region.String()),
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
	}
}

// standardQueriesComponent returns the standard DNS query component.
func (z *Route53Zone) standardQueriesComponent() query.Component {
	return query.Component{
		Name:            "Standard queries",
		MonthlyQuantity: z.monthlyStandardQueries,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(z.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Query"),
			Location: util.StringPtr(z.region.String()),
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
	}
}

// latencyQueriesComponent returns the latency-based routing DNS query component.
func (z *Route53Zone) latencyQueriesComponent() query.Component {
	return query.Component{
		Name:            "Latency-based queries",
		MonthlyQuantity: z.monthlyLatencyQueries,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(z.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Query"),
			Location: util.StringPtr(z.region.String()),
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
	}
}

// geoQueriesComponent returns the geolocation DNS query component.
func (z *Route53Zone) geoQueriesComponent() query.Component {
	return query.Component{
		Name:            "Geo queries",
		MonthlyQuantity: z.monthlyGeoQueries,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(z.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Query"),
			Location: util.StringPtr(z.region.String()),
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
	}
}
