package terraform_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	awstf "github.com/cycloidio/terracost/aws/terraform"
	"github.com/cycloidio/terracost/query"
	"github.com/cycloidio/terracost/terraform"
	"github.com/cycloidio/terracost/testutil"
)

func TestRoute53Record_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("SimpleRecord", func(t *testing.T) {
		// Simple A record — zero cost (configuration only)
		tfres := terraform.Resource{
			Address:      "aws_route53_record.simple",
			Type:         "aws_route53_record",
			Name:         "simple",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"zone_id": "Z1234567890",
				"name":    "example.com",
				"type":    "A",
			},
		}
		rss := map[string]terraform.Resource{}

		// Records are configuration-only — nil components (zero cost)
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, nil, actual)
	})

	t.Run("LatencyRecord", func(t *testing.T) {
		// Latency-based routing record — still zero cost
		tfres := terraform.Resource{
			Address:      "aws_route53_record.latency",
			Type:         "aws_route53_record",
			Name:         "latency",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"zone_id":       "Z1234567890",
				"name":          "app.example.com",
				"type":          "A",
				"set_identifier": "us-east-1",
				"latency_routing_policy": []map[string]interface{}{
					{"region": "us-east-1"},
				},
			},
		}
		rss := map[string]terraform.Resource{}

		actual := p.ResourceComponents(rss, tfres)
		// Should be nil — records don't emit cost components
		testutil.EqualQueryComponents(t, []query.Component(nil), actual)
	})

	t.Run("GeoRecord", func(t *testing.T) {
		// Geolocation routing record — still zero cost
		tfres := terraform.Resource{
			Address:      "aws_route53_record.geo",
			Type:         "aws_route53_record",
			Name:         "geo",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"zone_id":       "Z1234567890",
				"name":          "geo.example.com",
				"type":          "A",
				"set_identifier": "us",
				"geolocation_routing_policy": []map[string]interface{}{
					{"continent": "NA"},
				},
			},
		}
		rss := map[string]terraform.Resource{}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, []query.Component(nil), actual)
	})
}
