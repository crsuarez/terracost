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

func TestDynamoDBTable_Components(t *testing.T) {
	p, err := awstf.NewProvider("aws", "us-east-1")
	require.NoError(t, err)

	t.Run("OnDemand", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_dynamodb_table.test",
			Type:         "aws_dynamodb_table",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"billing_mode": "PAY_PER_REQUEST",
				"name":         "test-table",
			},
		}
		rss := map[string]terraform.Resource{}

		// Default usage: 1M reads, 200K writes, 50 GB storage, 50 GB PITR
		// Read/write request units use raw AWS PricePerUnit quantities.
		expected := []query.Component{
			{
				Name:            "Read request units (on-demand)",
				MonthlyQuantity: decimal.NewFromInt(1000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
						{Key: "UsageType", Value: util.StringPtr("ReadRequestUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Write request units (on-demand)",
				MonthlyQuantity: decimal.NewFromInt(200000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
						{Key: "UsageType", Value: util.StringPtr("WriteRequestUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("WriteRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Storage",
				MonthlyQuantity: decimal.NewFromInt(50),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
						{Key: "UsageType", Value: util.StringPtr("TimedStorage-ByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := usage.Default.GetUsage("aws_dynamodb_table")
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("Provisioned", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_dynamodb_table.test",
			Type:         "aws_dynamodb_table",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"billing_mode":   "PROVISIONED",
				"read_capacity":  10,
				"write_capacity": 5,
				"name":           "test-table",
			},
		}
		rss := map[string]terraform.Resource{}

		expected := []query.Component{
			{
				Name:           "Read capacity units (provisioned)",
				HourlyQuantity: decimal.NewFromInt(10),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
						{Key: "UsageType", Value: util.StringPtr("ReadCapacityUnit-Hrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadCapacityUnit-Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:           "Write capacity units (provisioned)",
				HourlyQuantity: decimal.NewFromInt(5),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
						{Key: "UsageType", Value: util.StringPtr("WriteCapacityUnit-Hrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("WriteCapacityUnit-Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Storage",
				MonthlyQuantity: decimal.NewFromInt(50),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
						{Key: "UsageType", Value: util.StringPtr("TimedStorage-ByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		us := usage.Default.GetUsage("aws_dynamodb_table")
		tfres.Values[usage.Key] = us
		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("WithPITR", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_dynamodb_table.test",
			Type:         "aws_dynamodb_table",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"billing_mode": "PAY_PER_REQUEST",
				"name":         "test-table",
				"point_in_time_recovery": []map[string]interface{}{
					{"enabled": true},
				},
			},
		}
		rss := map[string]terraform.Resource{}

		us := map[string]interface{}{
			"monthly_read_request_units":  1000000.0,
			"monthly_write_request_units": 200000.0,
			"storage_gb":                  50.0,
			"monthly_pitr_storage_gb":     50.0,
			"monthly_on_demand_backup_gb": 0.0,
			"monthly_stream_reads":        0.0,
		}
		tfres.Values[usage.Key] = us

		expected := []query.Component{
			{
				Name:            "Read request units (on-demand)",
				MonthlyQuantity: decimal.NewFromInt(1000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
						{Key: "UsageType", Value: util.StringPtr("ReadRequestUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Write request units (on-demand)",
				MonthlyQuantity: decimal.NewFromInt(200000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
						{Key: "UsageType", Value: util.StringPtr("WriteRequestUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("WriteRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Storage",
				MonthlyQuantity: decimal.NewFromInt(50),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
						{Key: "UsageType", Value: util.StringPtr("TimedStorage-ByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "PITR backup storage",
				MonthlyQuantity: decimal.NewFromInt(50),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-CrossRegionReplication")},
						{Key: "UsageType", Value: util.StringPtr("DDB-PITRStorageSnapshotByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("WithStream", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_dynamodb_table.test",
			Type:         "aws_dynamodb_table",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"billing_mode":     "PAY_PER_REQUEST",
				"name":             "test-table",
				"stream_enabled":   true,
				"stream_view_type": "NEW_AND_OLD_IMAGES",
			},
		}
		rss := map[string]terraform.Resource{}

		us := map[string]interface{}{
			"monthly_read_request_units":  1000000.0,
			"monthly_write_request_units": 200000.0,
			"storage_gb":                  50.0,
			"monthly_pitr_storage_gb":     0.0,
			"monthly_on_demand_backup_gb": 0.0,
			"monthly_stream_reads":        1000000.0,
		}
		tfres.Values[usage.Key] = us

		expected := []query.Component{
			{
				Name:            "Read request units (on-demand)",
				MonthlyQuantity: decimal.NewFromInt(1000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
						{Key: "UsageType", Value: util.StringPtr("ReadRequestUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Write request units (on-demand)",
				MonthlyQuantity: decimal.NewFromInt(200000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
						{Key: "UsageType", Value: util.StringPtr("WriteRequestUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("WriteRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Storage",
				MonthlyQuantity: decimal.NewFromInt(50),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
						{Key: "UsageType", Value: util.StringPtr("TimedStorage-ByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Stream read request units",
				MonthlyQuantity: decimal.NewFromInt(1000000),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StreamsEventDataUnits")},
						{Key: "UsageType", Value: util.StringPtr("DDB-Streams-ReadUnits")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadRequestUnit"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})

	t.Run("WithGSI", func(t *testing.T) {
		tfres := terraform.Resource{
			Address:      "aws_dynamodb_table.test",
			Type:         "aws_dynamodb_table",
			Name:         "test",
			ProviderName: "aws",
			Values: map[string]interface{}{
				"billing_mode":   "PROVISIONED",
				"read_capacity":  10,
				"write_capacity": 10,
				"name":           "test-table",
				"global_secondary_index": []map[string]interface{}{
					{
						"name":           "gsi1",
						"read_capacity":  5,
						"write_capacity": 5,
					},
				},
			},
		}
		rss := map[string]terraform.Resource{}

		us := map[string]interface{}{
			"monthly_read_request_units":  0.0,
			"monthly_write_request_units": 0.0,
			"storage_gb":                  25.0,
			"monthly_pitr_storage_gb":     0.0,
			"monthly_on_demand_backup_gb": 0.0,
			"monthly_stream_reads":        0.0,
		}
		tfres.Values[usage.Key] = us

		expected := []query.Component{
			{
				Name:           "Read capacity units (provisioned)",
				HourlyQuantity: decimal.NewFromInt(10),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
						{Key: "UsageType", Value: util.StringPtr("ReadCapacityUnit-Hrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadCapacityUnit-Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:           "Write capacity units (provisioned)",
				HourlyQuantity: decimal.NewFromInt(10),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
						{Key: "UsageType", Value: util.StringPtr("WriteCapacityUnit-Hrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("WriteCapacityUnit-Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "Storage",
				MonthlyQuantity: decimal.NewFromInt(25),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
						{Key: "UsageType", Value: util.StringPtr("TimedStorage-ByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:            "GSI gsi1 storage",
				MonthlyQuantity: decimal.NewFromInt(25),
				Usage:           true,
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
						{Key: "UsageType", Value: util.StringPtr("TimedStorage-ByteHrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("GB-Mo"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:           "GSI gsi1 read capacity",
				HourlyQuantity: decimal.NewFromInt(5),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
						{Key: "UsageType", Value: util.StringPtr("ReadCapacityUnit-Hrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("ReadCapacityUnit-Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
			{
				Name:           "GSI gsi1 write capacity",
				HourlyQuantity: decimal.NewFromInt(5),
				ProductFilter: &product.Filter{
					Provider: util.StringPtr("aws"),
					Service:  util.StringPtr("AmazonDynamoDB"),
					Location: util.StringPtr("us-east-1"),
					AttributeFilters: []*product.AttributeFilter{
						{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
						{Key: "UsageType", Value: util.StringPtr("WriteCapacityUnit-Hrs")},
					},
				},
				PriceFilter: &price.Filter{
					Unit: util.StringPtr("WriteCapacityUnit-Hrs"),
					AttributeFilters: []*price.AttributeFilter{
						{Key: "TermType", Value: util.StringPtr("OnDemand")},
					},
				},
			},
		}

		actual := p.ResourceComponents(rss, tfres)
		testutil.EqualQueryComponents(t, expected, actual)
	})
}
