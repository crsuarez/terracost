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
	lambdaDefaultMemorySize       = decimal.NewFromInt(128)
	lambdaDefaultEphemeralStorage = decimal.NewFromInt(512)
	lambdaEphemeralStorageFree    = decimal.NewFromInt(512)
	megabytesPerGigabyte          = decimal.NewFromInt(1024)
	msPerSecond                   = decimal.NewFromInt(1000)
	secondsPerMonth               = decimal.NewFromInt(2_678_400) // 31 days
)

// LambdaFunction represents an aws_lambda_function for cost estimation.
type LambdaFunction struct {
	provider *Provider
	region   region.Code

	architecture       string // "x86_64" or "arm64"
	memorySizeMB       decimal.Decimal
	ephemeralStorageMB decimal.Decimal

	// Usage (from tc_usage)
	monthlyRequests                      decimal.Decimal
	requestDurationMs                    decimal.Decimal
	monthlyProvisionedConcurrencySeconds decimal.Decimal
}

type lambdaFunctionValues struct {
	Architectures    []string `mapstructure:"architectures"`
	MemorySize       int64    `mapstructure:"memory_size"`
	EphemeralStorage []struct {
		Size int64 `mapstructure:"size"`
	} `mapstructure:"ephemeral_storage"`

	Usage struct {
		MonthlyRequests                      float64 `mapstructure:"monthly_requests"`
		RequestDurationMs                    float64 `mapstructure:"request_duration_ms"`
		MonthlyProvisionedConcurrencySeconds float64 `mapstructure:"monthly_provisioned_concurrency_seconds"`
	} `mapstructure:"tc_usage"`
}

// decodeLambdaFunctionValues decodes and returns lambdaFunctionValues from a Terraform values map.
func decodeLambdaFunctionValues(tfVals map[string]interface{}) (lambdaFunctionValues, error) {
	var v lambdaFunctionValues
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

// newLambdaFunction creates a new LambdaFunction from lambdaFunctionValues.
func (p *Provider) newLambdaFunction(_ map[string]terraform.Resource, vals lambdaFunctionValues) *LambdaFunction {
	lf := &LambdaFunction{
		provider:                             p,
		region:                               p.region,
		architecture:                         "x86_64",
		memorySizeMB:                         lambdaDefaultMemorySize,
		ephemeralStorageMB:                   lambdaDefaultEphemeralStorage,
		monthlyRequests:                      decimal.NewFromFloat(vals.Usage.MonthlyRequests),
		requestDurationMs:                    decimal.NewFromFloat(vals.Usage.RequestDurationMs),
		monthlyProvisionedConcurrencySeconds: decimal.NewFromFloat(vals.Usage.MonthlyProvisionedConcurrencySeconds),
	}

	if len(vals.Architectures) > 0 && vals.Architectures[0] != "" {
		lf.architecture = vals.Architectures[0]
	}

	if vals.MemorySize > 0 {
		lf.memorySizeMB = decimal.NewFromInt(vals.MemorySize)
	}

	if len(vals.EphemeralStorage) > 0 && vals.EphemeralStorage[0].Size > 0 {
		lf.ephemeralStorageMB = decimal.NewFromInt(int64(vals.EphemeralStorage[0].Size))
	}

	return lf
}

// Components returns the price component queries that make up this LambdaFunction.
func (lf *LambdaFunction) Components() []query.Component {
	components := []query.Component{
		lf.requestsComponent(),
		lf.durationComponent(),
	}

	if comp, ok := lf.ephemeralStorageComponent(); ok {
		components = append(components, comp)
	}

	if comp, ok := lf.provisionedConcurrencyComponent(); ok {
		components = append(components, comp)
	}

	return components
}

// architectureSuffix returns the AWS Lambda pricing suffix used by ARM rows.
func (lf *LambdaFunction) architectureSuffix() string {
	if lf.architecture == "arm64" {
		return "-ARM"
	}
	return ""
}

func (lf *LambdaFunction) architectureGroup(base string) string {
	return base + lf.architectureSuffix()
}

func (lf *LambdaFunction) requestsComponent() query.Component {
	return query.Component{
		Name:            "Requests",
		MonthlyQuantity: lf.monthlyRequests,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(lf.provider.key),
			Service:  util.StringPtr("AWSLambda"),
			Family:   util.StringPtr("Serverless"),
			Location: util.StringPtr(lf.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr("AWS-Lambda-Requests")},
				{Key: "UsageType", Value: util.StringPtr("Request")},
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

func (lf *LambdaFunction) durationComponent() query.Component {
	// GB-seconds = (memory_mb / 1024) * monthly_requests * (duration_ms / 1000)
	gbSeconds := lf.memorySizeMB.
		Div(megabytesPerGigabyte).
		Mul(lf.monthlyRequests).
		Mul(lf.requestDurationMs).
		Div(msPerSecond)

	return query.Component{
		Name:            "Duration",
		MonthlyQuantity: gbSeconds,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(lf.provider.key),
			Service:  util.StringPtr("AWSLambda"),
			Family:   util.StringPtr("Serverless"),
			Location: util.StringPtr(lf.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr(lf.architectureGroup("AWS-Lambda-Duration"))},
				{Key: "UsageType", Value: util.StringPtr("Lambda-GB-Second" + lf.architectureSuffix())},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("Lambda-GB-Second"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}
}

func (lf *LambdaFunction) ephemeralStorageComponent() (query.Component, bool) {
	// Only emit if ephemeral storage exceeds the 512 MB free tier
	additionalStorageMB := lf.ephemeralStorageMB.Sub(lambdaEphemeralStorageFree)
	if additionalStorageMB.LessThanOrEqual(decimal.Zero) {
		return query.Component{}, false
	}

	additionalStorageGB := additionalStorageMB.Div(megabytesPerGigabyte)
	// GB-seconds = additional_GB * monthly_requests * (duration_ms / 1000)
	gbSeconds := additionalStorageGB.
		Mul(lf.monthlyRequests).
		Mul(lf.requestDurationMs).
		Div(msPerSecond)

	return query.Component{
		Name:            "Ephemeral storage",
		MonthlyQuantity: gbSeconds,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(lf.provider.key),
			Service:  util.StringPtr("AWSLambda"),
			Family:   util.StringPtr("Serverless"),
			Location: util.StringPtr(lf.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr(lf.architectureGroup("AWS-Lambda-Storage-Duration"))},
				{Key: "UsageType", Value: util.StringPtr("Lambda-Storage-GB-Second" + lf.architectureSuffix())},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("GB-Seconds"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}, true
}

func (lf *LambdaFunction) provisionedConcurrencyComponent() (query.Component, bool) {
	if lf.monthlyProvisionedConcurrencySeconds.LessThanOrEqual(decimal.Zero) {
		return query.Component{}, false
	}

	// GB-seconds = (memory_mb / 1024) * provisioned_concurrency_seconds
	gbSeconds := lf.memorySizeMB.
		Div(megabytesPerGigabyte).
		Mul(lf.monthlyProvisionedConcurrencySeconds)

	return query.Component{
		Name:            "Provisioned concurrency",
		MonthlyQuantity: gbSeconds,
		Usage:           true,
		ProductFilter: &product.Filter{
			Provider: util.StringPtr(lf.provider.key),
			Service:  util.StringPtr("AWSLambda"),
			Family:   util.StringPtr("Serverless"),
			Location: util.StringPtr(lf.region.String()),
			AttributeFilters: []*product.AttributeFilter{
				{Key: "Group", Value: util.StringPtr(lf.architectureGroup("AWS-Lambda-Provisioned-Concurrency"))},
				{Key: "UsageType", Value: util.StringPtr("Lambda-Provisioned-Concurrency" + lf.architectureSuffix())},
			},
		},
		PriceFilter: &price.Filter{
			Unit: util.StringPtr("Lambda-GB-Second"),
			AttributeFilters: []*price.AttributeFilter{
				{Key: "TermType", Value: util.StringPtr("OnDemand")},
			},
		},
	}, true
}
