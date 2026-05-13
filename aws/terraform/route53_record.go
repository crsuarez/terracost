package terraform

import (
	"github.com/mitchellh/mapstructure"

	"github.com/cycloidio/terracost/query"
	"github.com/cycloidio/terracost/terraform"
)

// Route53Record represents an aws_route53_record for cost estimation.
//
// Route 53 records are primarily configuration — they do not have a direct per-record fee.
// The cost of DNS queries is modeled on the aws_route53_zone resource via tc_usage.
//
// This resource exists so that the provider dispatch recognizes the type and does not
// emit a nil/unknown resource. It returns an empty component list (zero cost).
type Route53Record struct {
	// Routing policy is informational; the zone's tc_usage determines query pricing.
	routingPolicy string // "simple", "weighted", "latency", "failover", "geolocation", "geoproximity", "multivalue"
}

type route53RecordValues struct {
	SetIdentifier              string `mapstructure:"set_identifier"`
	FailoverRoutingPolicy      []struct{} `mapstructure:"failover_routing_policy"`
	LatencyRoutingPolicy       []struct{} `mapstructure:"latency_routing_policy"`
	GeolocationRoutingPolicy   []struct{} `mapstructure:"geolocation_routing_policy"`
	GeoproximityRoutingPolicy  []struct{} `mapstructure:"geoproximity_routing_policy"`
	WeightedRoutingPolicy      []struct{} `mapstructure:"weighted_routing_policy"`
	MultivalueAnswerRoutingPolicy bool `mapstructure:"multivalue_answer_routing_policy"`
}

// decodeRoute53RecordValues decodes and returns route53RecordValues from a Terraform values map.
func decodeRoute53RecordValues(tfVals map[string]interface{}) (route53RecordValues, error) {
	var v route53RecordValues
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

// newRoute53Record creates a new Route53Record from route53RecordValues.
func (p *Provider) newRoute53Record(_ map[string]terraform.Resource, vals route53RecordValues) *Route53Record {
	rec := &Route53Record{
		routingPolicy: "simple",
	}

	// Determine routing policy from the record's fields.
	if len(vals.LatencyRoutingPolicy) > 0 {
		rec.routingPolicy = "latency"
	} else if len(vals.GeolocationRoutingPolicy) > 0 {
		rec.routingPolicy = "geolocation"
	} else if len(vals.GeoproximityRoutingPolicy) > 0 {
		rec.routingPolicy = "geoproximity"
	} else if len(vals.FailoverRoutingPolicy) > 0 {
		rec.routingPolicy = "failover"
	} else if len(vals.WeightedRoutingPolicy) > 0 {
		rec.routingPolicy = "weighted"
	} else if vals.MultivalueAnswerRoutingPolicy {
		rec.routingPolicy = "multivalue"
	}

	return rec
}

// Components returns the price component queries for this Route53Record.
// Records are configuration-only and carry zero direct cost.
func (r *Route53Record) Components() []query.Component {
	return nil
}
