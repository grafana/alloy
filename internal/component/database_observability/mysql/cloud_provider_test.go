package mysql

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/grafana/alloy/internal/component/database_observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPopulateCloudProvider(t *testing.T) {
	t.Run("populates AWS RDS details from DSN", func(t *testing.T) {
		dsn := "user:pass@tcp(products-db.abc123xyz.us-east-1.rds.amazonaws.com:3306)/schema"
		got, err := populateCloudProviderFromDSN(dsn)
		require.NoError(t, err)

		assert.Equal(t, &database_observability.CloudProvider{
			AWS: &database_observability.AWSCloudProviderInfo{
				ARN: arn.ARN{Resource: "db:products-db", Region: "us-east-1", AccountID: "unknown"},
			},
		}, got)
	})

	t.Run("populates Azure details from DSN", func(t *testing.T) {
		dsn := "user:pass@tcp(products-db.mysql.database.azure.com:3306)/schema"
		got, err := populateCloudProviderFromDSN(dsn)
		require.NoError(t, err)

		assert.Equal(t, &database_observability.CloudProvider{
			Azure: &database_observability.AzureCloudProviderInfo{
				ServerName: "products-db",
			},
		}, got)
	})
}
