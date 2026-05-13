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
	// httpTierBoundary is the boundary between the first tier (cheaper) and
	// the second tier of HTTP API pricing, in millions of requests.
	httpTierBoundary = decimal.NewFromInt(300_000_000)

	// connectionMinutesPerMonth is used for WebSocket connection-minute pricing.
	connectionMinutesPerMonth = decimal.NewFromInt(1)
)

// Apigatewayv2API represents an aws_apigatewayv2_api for cost estimation.
type Apigatewayv2API struct {
	provider *Provider
	region   region.Code

	protocolType string // "HTTP" or "WEBSOCKET"

	// Usage (from tc_usage)
	monthlyRequests       decimal.Decimal
	monthlyMessageCount   decimal.Decimal
	monthlyConnectionMins decimal.Decimal
}

type apigatewayv2APIValues struct {
	ProtocolType string `mapstructure:"protocol_type"`

	Usage struct {
		MonthlyRequests          float64 `mapstructure:"monthly_requests"`
		MonthlyMessageCount      float64 `mapstructure:"monthly_message_count"`
		MonthlyConnectionMinutes float64 `mapstructure:"monthly_connection_minutes"`
	} `mapstructure:"tc_usage"`
}

// decodeApigatewayv2APIValues decodes and returns apigatewayv2APIValues from a Terraform values map.
func decodeApigatewayv2APIValues(tfVals map[string]interface{}) (apigatewayv2APIValues, error) {
	var v apigatewayv2APIValues
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

// newApigatewayv2API creates a new Apigatewayv2API from apigatewayv2APIValues.
func (p *Provider) newApigatewayv2API(_ map[string]terraform.Resource, vals apigatewayv2APIValues) *Apigatewayv2API {
	api := &Apigatewayv2API{
		provider:              p,
		region:                p.region,
		protocolType:          "HTTP", // AWS default
		monthlyRequests:       decimal.NewFromFloat(vals.Usage.MonthlyRequests),
		monthlyMessageCount:   decimal.NewFromFloat(vals.Usage.MonthlyMessageCount),
		monthlyConnectionMins: decimal.NewFromFloat(vals.Usage.MonthlyConnectionMinutes),
	}

	if vals.ProtocolType != "" {
		api.protocolType = vals.ProtocolType
	}

	return api
}

// Components returns the price component queries that make up this Apigatewayv2API.
func (api *Apigatewayv2API) Components() []query.Component {
	if api.protocolType == "WEBSOCKET" {
		return api.websocketComponents()
	}
	return api.httpComponents()
}

// httpComponents returns components for an HTTP API with tiered pricing.
func (api *Apigatewayv2API) httpComponents() []query.Component {
	var components []query.Component

	if api.monthlyRequests.LessThanOrEqual(httpTierBoundary) {
		// All requests fall in the first (cheaper) tier
		components = append(components, api.httpRequestsComponent("0", api.monthlyRequests))
	} else {
		// First 300M at cheaper rate, remainder at standard rate
		components = append(components, api.httpRequestsComponent("0", httpTierBoundary))
		extraRequests := api.monthlyRequests.Sub(httpTierBoundary)
		components = append(components, api.httpRequestsComponent("300000000", extraRequests))
	}

	return components
}

// websocketComponents returns components for a WebSocket API.
func (api *Apigatewayv2API) websocketComponents() []query.Component {
	components := []query.Component{
		api.websocketMessageComponent(),
	}

	if api.monthlyConnectionMins.GreaterThan(decimal.Zero) {
		components = append(components, api.websocketConnectionComponent())
	}

	return components
}

func (api *Apigatewayv2API) httpRequestsComponent(startingRange string, quantity decimal.Decimal) query.Component {
	return query.Component{
		Name:            "HTTP API requests",
		MonthlyQuantity: quantity,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(api.provider.key),
			Service:  util.StringPtr("AmazonApiGateway"),
			Location: util.StringPtr(api.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("ApiGatewayHttpRequest")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("Requests"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
				{Key: "StartingRange", Value: util.StringPtr(startingRange)},
			},
		},
	}
}

func (api *Apigatewayv2API) websocketMessageComponent() query.Component {
	return query.Component{
		Name:            "WebSocket messages",
		MonthlyQuantity: api.monthlyMessageCount,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(api.provider.key),
			Service:  util.StringPtr("AmazonApiGateway"),
			Location: util.StringPtr(api.region.String()),
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
	}
}

func (api *Apigatewayv2API) websocketConnectionComponent() query.Component {
	return query.Component{
		Name:            "WebSocket connection minutes",
		MonthlyQuantity: api.monthlyConnectionMins,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(api.provider.key),
			Service:  util.StringPtr("AmazonApiGateway"),
			Location: util.StringPtr(api.region.String()),
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
	}
}
