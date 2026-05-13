package e2e

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	costestimation "github.com/cycloidio/terracost"
	"github.com/cycloidio/terracost/aws"
	"github.com/cycloidio/terracost/mock"
	"github.com/cycloidio/terracost/mysql"
	"github.com/cycloidio/terracost/product"
	"github.com/cycloidio/terracost/util"
)

func TestAWSIngestion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	httpClient := mock.NewHTTPClient(ctrl)

	db, err := sql.Open("mysql", testDSN())
	require.NoError(t, err)

	f, err := os.Open("testdata/AmazonEC2_eu-west-3.csv")
	require.NoError(t, err)
	defer f.Close()

	httpClient.EXPECT().Do(gomock.Any()).Return(&http.Response{Body: f}, nil)

	backend := mysql.NewBackend(db)
	ingester, err := aws.NewIngester("AmazonEC2", "eu-west-3", aws.WithHTTPClient(httpClient))
	require.NoError(t, err)

	err = costestimation.IngestPricing(ctx, backend, ingester)
	require.NoError(t, err)

	allProds, err := backend.Products().Filter(ctx, &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonEC2-test")})
	require.NoError(t, err)
	assert.Len(t, allProds, 5)

	for _, prod := range allProds {
		prices, err := backend.Prices().Filter(ctx, prod.ID, nil)
		require.NoError(t, err)
		assert.Len(t, prices, 1)
	}

	t.Run("Lambda", func(t *testing.T) {
		lambdaCtrl := gomock.NewController(t)
		defer lambdaCtrl.Finish()

		lambdaHTTPClient := mock.NewHTTPClient(lambdaCtrl)

		lambdaF, err := os.Open("testdata/aws/lambda/AWSLambda_us-east-1.csv")
		require.NoError(t, err)
		defer lambdaF.Close()

		lambdaHTTPClient.EXPECT().Do(gomock.Any()).Return(&http.Response{Body: lambdaF}, nil)

		lambdaBackend := mysql.NewBackend(db)
		lambdaIngester, err := aws.NewIngester("AWSLambda", "us-east-1", aws.WithHTTPClient(lambdaHTTPClient))
		require.NoError(t, err)

		err = costestimation.IngestPricing(ctx, lambdaBackend, lambdaIngester)
		require.NoError(t, err)

		lambdaProds, err := lambdaBackend.Products().Filter(ctx, &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AWSLambda")})
		require.NoError(t, err)
		assert.Len(t, lambdaProds, 5)

		for _, prod := range lambdaProds {
			prices, err := lambdaBackend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
		assertIngestedSKU(t, ctx, lambdaBackend, "aws", "GU2ZS9HVP6QTQ7KE", "0.0000002000")
		assertIngestedSKU(t, ctx, lambdaBackend, "aws", "DECOYLAMBDASAVINGS", "9.9900000000")
	})
	t.Run("DynamoDB", func(t *testing.T) {
		dynCtrl := gomock.NewController(t)
		defer dynCtrl.Finish()

		dynHTTPClient := mock.NewHTTPClient(dynCtrl)

		dynF, err := os.Open("testdata/aws/dynamodb/AmazonDynamoDB_us-east-1.csv")
		require.NoError(t, err)
		defer dynF.Close()

		dynHTTPClient.EXPECT().Do(gomock.Any()).Return(&http.Response{Body: dynF}, nil)

		dynBackend := mysql.NewBackend(db)
		dynIngester, err := aws.NewIngester("AmazonDynamoDB", "us-east-1", aws.WithHTTPClient(dynHTTPClient))
		require.NoError(t, err)

		err = costestimation.IngestPricing(ctx, dynBackend, dynIngester)
		require.NoError(t, err)

		dynProds, err := dynBackend.Products().Filter(ctx, &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonDynamoDB")})
		require.NoError(t, err)
		assert.Len(t, dynProds, 9)

		for _, prod := range dynProds {
			prices, err := dynBackend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
		assertIngestedSKU(t, ctx, dynBackend, "aws", "4W4ZMC46EHE8XTTZ", "0.0000002500")
		assertIngestedSKU(t, ctx, dynBackend, "aws", "DECOYDDBRCU", "9.9900000000")
	})
	t.Run("APIGateway", func(t *testing.T) {
		apigwCtrl := gomock.NewController(t)
		defer apigwCtrl.Finish()

		apigwHTTPClient := mock.NewHTTPClient(apigwCtrl)

		apigwF, err := os.Open("testdata/aws/api_gateway/AmazonApiGateway_us-east-1.csv")
		require.NoError(t, err)
		defer apigwF.Close()

		apigwHTTPClient.EXPECT().Do(gomock.Any()).Return(&http.Response{Body: apigwF}, nil)

		apigwBackend := mysql.NewBackend(db)
		apigwIngester, err := aws.NewIngester("AmazonApiGateway", "us-east-1", aws.WithHTTPClient(apigwHTTPClient))
		require.NoError(t, err)

		err = costestimation.IngestPricing(ctx, apigwBackend, apigwIngester)
		require.NoError(t, err)

		apigwProds, err := apigwBackend.Products().Filter(ctx, &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonApiGateway")})
		require.NoError(t, err)
		assert.Len(t, apigwProds, 7)

		for _, prod := range apigwProds {
			prices, err := apigwBackend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
		assertIngestedSKU(t, ctx, apigwBackend, "aws", "FC2TWT2UEPTBKVBX", "0.0000010000")
		assertIngestedSKU(t, ctx, apigwBackend, "aws", "DECOYHTTPRESTOP", "9.9900000000")
	})
	t.Run("CloudFront", func(t *testing.T) {
		cfCtrl := gomock.NewController(t)
		defer cfCtrl.Finish()

		cfHTTPClient := mock.NewHTTPClient(cfCtrl)

		cfF, err := os.Open("testdata/aws/cloudfront/AmazonCloudFront_us-east-1.csv")
		require.NoError(t, err)
		defer cfF.Close()

		cfHTTPClient.EXPECT().Do(gomock.Any()).Return(&http.Response{Body: cfF}, nil)

		cfBackend := mysql.NewBackend(db)
		cfIngester, err := aws.NewIngester("AmazonCloudFront", "us-east-1", aws.WithHTTPClient(cfHTTPClient))
		require.NoError(t, err)

		err = costestimation.IngestPricing(ctx, cfBackend, cfIngester)
		require.NoError(t, err)

		cfProds, err := cfBackend.Products().Filter(ctx, &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonCloudFront")})
		require.NoError(t, err)
		assert.Len(t, cfProds, 7)

		for _, prod := range cfProds {
			prices, err := cfBackend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
		assertIngestedSKU(t, ctx, cfBackend, "aws", "CFREQHTTPS001", "0.0100000000")
		assertIngestedSKU(t, ctx, cfBackend, "aws", "DECOYCFHTTPS", "9.9900000000")
	})
	t.Run("Route53", func(t *testing.T) {
		r53Ctrl := gomock.NewController(t)
		defer r53Ctrl.Finish()

		r53HTTPClient := mock.NewHTTPClient(r53Ctrl)

		r53F, err := os.Open("testdata/aws/route53/AmazonRoute53_us-east-1.csv")
		require.NoError(t, err)
		defer r53F.Close()

		r53HTTPClient.EXPECT().Do(gomock.Any()).Return(&http.Response{Body: r53F}, nil)

		r53Backend := mysql.NewBackend(db)
		r53Ingester, err := aws.NewIngester("AmazonRoute53", "us-east-1", aws.WithHTTPClient(r53HTTPClient))
		require.NoError(t, err)

		err = costestimation.IngestPricing(ctx, r53Backend, r53Ingester)
		require.NoError(t, err)

		r53Prods, err := r53Backend.Products().Filter(ctx, &product.Filter{Provider: util.StringPtr("aws"), Service: util.StringPtr("AmazonRoute53")})
		require.NoError(t, err)
		assert.Len(t, r53Prods, 9)

		for _, prod := range r53Prods {
			prices, err := r53Backend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
		assertIngestedSKU(t, ctx, r53Backend, "aws", "STDQTEST001", "0.0000004000")
		assertIngestedSKU(t, ctx, r53Backend, "aws", "DECOYR53QUERY", "9.9900000000")
	})
}

func assertIngestedSKU(t *testing.T, ctx context.Context, backend *mysql.Backend, provider, sku string, wantPrice string) {
	t.Helper()

	prod, err := backend.Products().FindByVendorAndSKU(ctx, provider, sku)
	require.NoError(t, err)

	prices, err := backend.Prices().Filter(ctx, prod.ID, nil)
	require.NoError(t, err)
	require.Len(t, prices, 1)
	assert.Equal(t, wantPrice, prices[0].Value.StringFixed(10))
}
