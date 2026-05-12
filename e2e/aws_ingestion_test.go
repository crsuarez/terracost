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

	db, err := sql.Open("mysql", "root:terracost@tcp(172.44.0.2:3306)/terracost_test?multiStatements=true")
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
		assert.Len(t, lambdaProds, 4)

		for _, prod := range lambdaProds {
			prices, err := lambdaBackend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
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
		assert.Len(t, dynProds, 8)

		for _, prod := range dynProds {
			prices, err := dynBackend.Prices().Filter(ctx, prod.ID, nil)
			require.NoError(t, err)
			assert.Len(t, prices, 1)
		}
	})
}
