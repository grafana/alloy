package database_observability

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateCloudProvider(t *testing.T) {
	t.Run("returns pre-populated cloud provider as-is", func(t *testing.T) {
		csp := &CloudProvider{
			AWS: &AWSCloudProviderInfo{
				ARN: arn.ARN{Resource: "some-resource", Region: "some-region", AccountID: "some-account"},
			},
		}

		got, err := PopulateCloudProvider(csp, "some-dsn")
		require.NoError(t, err)

		assert.Equal(t, csp, got)
	})

	t.Run("populates AWS RDS details from DSN", func(t *testing.T) {
		dsn := "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/schema"
		got, err := PopulateCloudProvider(nil, dsn)
		require.NoError(t, err)

		assert.Equal(t, &CloudProvider{
			AWS: &AWSCloudProviderInfo{
				ARN: arn.ARN{Resource: "db:products-db", Region: "us-east-1", AccountID: "unknown"},
			},
		}, got)
	})

	t.Run("populates Azure details from DSN", func(t *testing.T) {
		dsn := "user:pass@tcp(products-db.mysql.database.azure.com:3306)/schema"
		got, err := PopulateCloudProvider(nil, dsn)
		require.NoError(t, err)

		assert.Equal(t, &CloudProvider{
			Azure: &AzureCloudProviderInfo{
				Resource: "products-db",
			},
		}, got)
	})
}
