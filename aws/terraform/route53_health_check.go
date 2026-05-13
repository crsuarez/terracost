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

// Route53HealthCheck represents an aws_route53_health_check for cost estimation.
//
// Pricing dimensions (per AWS Route 53 pricing page):
//   - Basic health check (AWS endpoint): $0.50 per check per month.
//   - Custom endpoint health check: $0.75 per check per month.
//   - Optional features (each adds a per-check-month fee):
//   - HTTPS: +$0.50
//   - String matching (search_string non-empty): +$0.50
//   - Latency measurement (measure_latency = true): +$0.50
//   - Fast interval (request_interval = 10): +$1.00 (standard is 30s)
//   - SNI (enable_sni = true): +$0.50
//
// The "type" field determines the base price:
//   - "HTTP" | "TCP" | "HTTP_STR_MATCH" | "TCP_STR_MATCH" with fqdn matching
//     an AWS resource → basic ($0.50). In practice, we classify by inspecting the
//     type prefix: types starting with "CALCULATED" or ending with "_STR_MATCH"
//     are treated as custom ($0.75) when they target non-AWS endpoints.
//   - For simplicity, we use the following heuristic aligned with AWS pricing:
//   - type == "HTTP" or "TCP" with no additional features → basic ($0.50).
//   - All other types → custom ($0.75).
//   - Feature add-ons stack on top.
type Route53HealthCheck struct {
	provider *Provider
	region   region.Code

	healthCheckType string // "HTTP", "TCP", "HTTPS", "HTTP_STR_MATCH", "TCP_STR_MATCH", "CALCULATED"

	// Feature flags
	measureLatency  bool
	enableSNI       bool
	searchStringSet bool
	fastInterval    bool // request_interval == 10
}

type route53HealthCheckValues struct {
	Type string `mapstructure:"type"`

	RequestInterval int64  `mapstructure:"request_interval"`
	MeasureLatency  bool   `mapstructure:"measure_latency"`
	EnableSNI       bool   `mapstructure:"enable_sni"`
	SearchString    string `mapstructure:"search_string"`
}

// decodeRoute53HealthCheckValues decodes and returns route53HealthCheckValues from a Terraform values map.
func decodeRoute53HealthCheckValues(tfVals map[string]interface{}) (route53HealthCheckValues, error) {
	var v route53HealthCheckValues
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

// newRoute53HealthCheck creates a new Route53HealthCheck from route53HealthCheckValues.
func (p *Provider) newRoute53HealthCheck(_ map[string]terraform.Resource, vals route53HealthCheckValues) *Route53HealthCheck {
	hc := &Route53HealthCheck{
		provider:        p,
		region:          p.region,
		healthCheckType: vals.Type,
		measureLatency:  vals.MeasureLatency,
		enableSNI:       vals.EnableSNI,
		searchStringSet: vals.SearchString != "",
		fastInterval:    vals.RequestInterval == 10,
	}

	return hc
}

// Components returns the price component queries that make up this Route53HealthCheck.
func (hc *Route53HealthCheck) Components() []query.Component {
	components := []query.Component{
		hc.baseHealthCheckComponent(),
	}

	if hc.enableSNI {
		components = append(components, hc.sniComponent())
	}
	if hc.searchStringSet {
		components = append(components, hc.stringMatchingComponent())
	}
	if hc.measureLatency {
		components = append(components, hc.latencyMeasurementComponent())
	}
	if hc.fastInterval {
		components = append(components, hc.fastIntervalComponent())
	}

	return components
}

// isBasic returns true if the health check targets a basic AWS endpoint.
// Basic checks use HTTP or TCP without additional string matching features.
func (hc *Route53HealthCheck) isBasic() bool {
	switch hc.healthCheckType {
	case "HTTP", "TCP":
		return true
	default:
		return false
	}
}

// baseHealthCheckComponent returns the base health check component.
func (hc *Route53HealthCheck) baseHealthCheckComponent() query.Component {
	group := "HealthCheck-Custom-Endpoint"
	priceDesc := "Custom health check"

	if hc.isBasic() {
		group = "HealthCheck-AWS-Endpoint"
		priceDesc = "Basic health check (AWS endpoint)"
	}

	return query.Component{
		Name:            priceDesc,
		MonthlyQuantity: decimal.NewFromInt(1),
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(hc.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Health Check"),
			Location: util.StringPtr(hc.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr(group)},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("HealthCheck"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// sniComponent returns the SNI feature add-on component.
func (hc *Route53HealthCheck) sniComponent() query.Component {
	return query.Component{
		Name:            "SNI",
		MonthlyQuantity: decimal.NewFromInt(1),
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(hc.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Health Check"),
			Location: util.StringPtr(hc.region.String()),
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
	}
}

// stringMatchingComponent returns the string matching feature add-on component.
func (hc *Route53HealthCheck) stringMatchingComponent() query.Component {
	return query.Component{
		Name:            "String matching",
		MonthlyQuantity: decimal.NewFromInt(1),
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(hc.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Health Check"),
			Location: util.StringPtr(hc.region.String()),
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
	}
}

// latencyMeasurementComponent returns the latency measurement feature add-on component.
func (hc *Route53HealthCheck) latencyMeasurementComponent() query.Component {
	return query.Component{
		Name:            "Latency measurement",
		MonthlyQuantity: decimal.NewFromInt(1),
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(hc.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Health Check"),
			Location: util.StringPtr(hc.region.String()),
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
	}
}

// fastIntervalComponent returns the fast interval (10s) feature add-on component.
func (hc *Route53HealthCheck) fastIntervalComponent() query.Component {
	return query.Component{
		Name:            "Fast interval",
		MonthlyQuantity: decimal.NewFromInt(1),
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(hc.provider.key),
			Service:  util.StringPtr("AmazonRoute53"),
			Family:   util.StringPtr("DNS Health Check"),
			Location: util.StringPtr(hc.region.String()),
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
	}
}
