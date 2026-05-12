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

// DynamoDBTable represents an aws_dynamodb_table for cost estimation.
type DynamoDBTable struct {
	provider *Provider
	region   region.Code

	billingMode string // "PAY_PER_REQUEST" or "PROVISIONED"

	// Provisioned capacity (used only when billing_mode == "PROVISIONED")
	readCapacity  decimal.Decimal
	writeCapacity decimal.Decimal

	// Feature flags
	pointInTimeRecoveryEnabled bool
	streamEnabled              bool

	// Global Secondary Indexes
	globalSecondaryIndexes []dynamodbGSI

	// Usage (from tc_usage)
	monthlyReadRequestUnits  decimal.Decimal
	monthlyWriteRequestUnits decimal.Decimal
	storageGB                decimal.Decimal
	monthlyPITRStorageGB     decimal.Decimal
	monthlyOnDemandBackupGB  decimal.Decimal
	monthlyStreamReads       decimal.Decimal
}

// dynamodbGSI represents a Global Secondary Index on a DynamoDB table.
type dynamodbGSI struct {
	name         string
	readCapacity  decimal.Decimal
	writeCapacity decimal.Decimal
	storageGB     decimal.Decimal
}

type dynamodbTableValues struct {
	BillingMode string `mapstructure:"billing_mode"`
	ReadCapacity  int64 `mapstructure:"read_capacity"`
	WriteCapacity int64 `mapstructure:"write_capacity"`

	PointInTimeRecovery []struct {
		Enabled bool `mapstructure:"enabled"`
	} `mapstructure:"point_in_time_recovery"`

	StreamEnabled   bool   `mapstructure:"stream_enabled"`
	StreamViewType  string `mapstructure:"stream_view_type"`

	GlobalSecondaryIndex []struct {
		Name         string `mapstructure:"name"`
		ReadCapacity  int64 `mapstructure:"read_capacity"`
		WriteCapacity int64 `mapstructure:"write_capacity"`
	} `mapstructure:"global_secondary_index"`

	Usage struct {
		MonthlyReadRequestUnits  float64 `mapstructure:"monthly_read_request_units"`
		MonthlyWriteRequestUnits float64 `mapstructure:"monthly_write_request_units"`
		StorageGB                float64 `mapstructure:"storage_gb"`
		MonthlyPITRStorageGB     float64 `mapstructure:"monthly_pitr_storage_gb"`
		MonthlyOnDemandBackupGB  float64 `mapstructure:"monthly_on_demand_backup_gb"`
		MonthlyStreamReads       float64 `mapstructure:"monthly_stream_reads"`
	} `mapstructure:"tc_usage"`
}

// decodeDynamoDBTableValues decodes and returns dynamodbTableValues from a Terraform values map.
func decodeDynamoDBTableValues(tfVals map[string]interface{}) (dynamodbTableValues, error) {
	var v dynamodbTableValues
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

// newDynamoDBTable creates a new DynamoDBTable from dynamodbTableValues.
func (p *Provider) newDynamoDBTable(_ map[string]terraform.Resource, vals dynamodbTableValues) *DynamoDBTable {
	dt := &DynamoDBTable{
		provider:    p,
		region:      p.region,
		billingMode: "PAY_PER_REQUEST", // AWS default

		// Usage
		monthlyReadRequestUnits:  decimal.NewFromFloat(vals.Usage.MonthlyReadRequestUnits),
		monthlyWriteRequestUnits: decimal.NewFromFloat(vals.Usage.MonthlyWriteRequestUnits),
		storageGB:                decimal.NewFromFloat(vals.Usage.StorageGB),
		monthlyPITRStorageGB:     decimal.NewFromFloat(vals.Usage.MonthlyPITRStorageGB),
		monthlyOnDemandBackupGB:  decimal.NewFromFloat(vals.Usage.MonthlyOnDemandBackupGB),
		monthlyStreamReads:       decimal.NewFromFloat(vals.Usage.MonthlyStreamReads),
	}

	if vals.BillingMode != "" {
		dt.billingMode = vals.BillingMode
	}

	if vals.ReadCapacity > 0 {
		dt.readCapacity = decimal.NewFromInt(vals.ReadCapacity)
	}
	if vals.WriteCapacity > 0 {
		dt.writeCapacity = decimal.NewFromInt(vals.WriteCapacity)
	}

	if len(vals.PointInTimeRecovery) > 0 {
		dt.pointInTimeRecoveryEnabled = vals.PointInTimeRecovery[0].Enabled
	}

	dt.streamEnabled = vals.StreamEnabled

	for _, gsi := range vals.GlobalSecondaryIndex {
		dt.globalSecondaryIndexes = append(dt.globalSecondaryIndexes, dynamodbGSI{
			name:          gsi.Name,
			readCapacity:  decimal.NewFromInt(gsi.ReadCapacity),
			writeCapacity: decimal.NewFromInt(gsi.WriteCapacity),
			storageGB:     dt.storageGB, // GSI storage mirrors base table for estimation
		})
	}

	return dt
}

// Components returns the price component queries that make up this DynamoDBTable.
func (dt *DynamoDBTable) Components() []query.Component {
	var components []query.Component

	if dt.billingMode == "PAY_PER_REQUEST" {
		components = append(components,
			dt.readRequestUnitsComponent(),
			dt.writeRequestUnitsComponent(),
		)
	} else {
		// PROVISIONED
		components = append(components,
			dt.provisionedReadComponent(),
			dt.provisionedWriteComponent(),
		)
	}

	components = append(components, dt.storageComponent())

	if dt.pointInTimeRecoveryEnabled {
		components = append(components, dt.pitrComponent())
	}

	if dt.monthlyOnDemandBackupGB.GreaterThan(decimal.Zero) {
		components = append(components, dt.onDemandBackupComponent())
	}

	if dt.streamEnabled {
		components = append(components, dt.streamComponent())
	}

	for _, gsi := range dt.globalSecondaryIndexes {
		components = append(components, dt.gsiStorageComponent(gsi))
		if dt.billingMode == "PROVISIONED" {
			components = append(components, dt.gsiReadComponent(gsi))
			components = append(components, dt.gsiWriteComponent(gsi))
		}
	}

	return components
}

// readRequestUnitsComponent returns the on-demand read request units component.
func (dt *DynamoDBTable) readRequestUnitsComponent() query.Component {
	return query.Component{
		Name:            "Read request units (on-demand)",
		MonthlyQuantity: dt.monthlyReadRequestUnits.Div(invocationsPerMillion),
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("ReadRequestUnit"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// writeRequestUnitsComponent returns the on-demand write request units component.
func (dt *DynamoDBTable) writeRequestUnitsComponent() query.Component {
	return query.Component{
		Name:            "Write request units (on-demand)",
		MonthlyQuantity: dt.monthlyWriteRequestUnits.Div(invocationsPerMillion),
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("WriteRequestUnit"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// provisionedReadComponent returns the provisioned read capacity component.
func (dt *DynamoDBTable) provisionedReadComponent() query.Component {
	return query.Component{
		Name:            "Read capacity units (provisioned)",
		HourlyQuantity:  dt.readCapacity,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("ReadCapacityUnit-Hrs"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// provisionedWriteComponent returns the provisioned write capacity component.
func (dt *DynamoDBTable) provisionedWriteComponent() query.Component {
	return query.Component{
		Name:            "Write capacity units (provisioned)",
		HourlyQuantity:  dt.writeCapacity,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("WriteCapacityUnit-Hrs"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// storageComponent returns the storage component.
func (dt *DynamoDBTable) storageComponent() query.Component {
	return query.Component{
		Name:            "Storage",
		MonthlyQuantity: dt.storageGB,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("GB-Mo"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// pitrComponent returns the point-in-time recovery (continuous backup) component.
func (dt *DynamoDBTable) pitrComponent() query.Component {
	return query.Component{
		Name:            "PITR backup storage",
		MonthlyQuantity: dt.monthlyPITRStorageGB,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
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
	}
}

// onDemandBackupComponent returns the on-demand backup component.
func (dt *DynamoDBTable) onDemandBackupComponent() query.Component {
	return query.Component{
		Name:            "On-demand backup storage",
		MonthlyQuantity: dt.monthlyOnDemandBackupGB,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
				{Key: "UsageType", Value: util.StringPtr("DDB-BackupStorageBytes")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("GB-Mo"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// streamComponent returns the DynamoDB Streams component.
func (dt *DynamoDBTable) streamComponent() query.Component {
	return query.Component{
		Name:            "Stream read request units",
		MonthlyQuantity: dt.monthlyStreamReads.Div(invocationsPerMillion),
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-StreamsEventDataUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("ReadRequestUnit"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// gsiStorageComponent returns the storage component for a GSI.
func (dt *DynamoDBTable) gsiStorageComponent(gsi dynamodbGSI) query.Component {
	return query.Component{
		Name:            "GSI " + gsi.name + " storage",
		MonthlyQuantity: gsi.storageGB,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-StorageUsage")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("GB-Mo"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// gsiReadComponent returns the provisioned read capacity component for a GSI.
func (dt *DynamoDBTable) gsiReadComponent(gsi dynamodbGSI) query.Component {
	return query.Component{
		Name:           "GSI " + gsi.name + " read capacity",
		HourlyQuantity: gsi.readCapacity,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-ReadUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("ReadCapacityUnit-Hrs"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

// gsiWriteComponent returns the provisioned write capacity component for a GSI.
func (dt *DynamoDBTable) gsiWriteComponent(gsi dynamodbGSI) query.Component {
	return query.Component{
		Name:           "GSI " + gsi.name + " write capacity",
		HourlyQuantity: gsi.writeCapacity,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(dt.provider.key),
			Service:  util.StringPtr("AmazonDynamoDB"),
			Location: util.StringPtr(dt.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("DDB-WriteUnits")},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("WriteCapacityUnit-Hrs"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}
