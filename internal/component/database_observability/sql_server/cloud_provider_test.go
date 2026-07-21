package sql_server

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/database_observability"
)

func TestPopulateCloudProviderFromDSN(t *testing.T) {
	t.Run("populates AWS RDS details from DSN", func(t *testing.T) {
		dsn := "sqlserver://user:pass@products-db.abc123xyz.us-east-1.rds.amazonaws.com:1433"
		got, err := populateCloudProviderFromDSN(dsn)
		require.NoError(t, err)

		assert.Equal(t, &database_observability.CloudProvider{
			AWS: &database_observability.AWSCloudProviderInfo{
				ARN: arn.ARN{Resource: "db:products-db", Region: "us-east-1", AccountID: "unknown"},
			},
		}, got)
	})

	t.Run("populates Azure details from DSN", func(t *testing.T) {
		dsn := "sqlserver://user:pass@products-db.database.windows.net:1433"
		got, err := populateCloudProviderFromDSN(dsn)
		require.NoError(t, err)

		assert.Equal(t, &database_observability.CloudProvider{
			Azure: &database_observability.AzureCloudProviderInfo{
				ServerName: "products-db",
			},
		}, got)
	})

	t.Run("populates Azure details from Managed Instance DSN", func(t *testing.T) {
		dsn := "sqlserver://user:pass@products-db.abc123.database.windows.net:1433"
		got, err := populateCloudProviderFromDSN(dsn)
		require.NoError(t, err)

		assert.Equal(t, &database_observability.CloudProvider{
			Azure: &database_observability.AzureCloudProviderInfo{
				ServerName: "products-db",
			},
		}, got)
	})

	t.Run("returns empty for a generic DSN", func(t *testing.T) {
		dsn := "sqlserver://user:pass@localhost:1433"
		got, err := populateCloudProviderFromDSN(dsn)
		require.NoError(t, err)

		assert.Equal(t, &database_observability.CloudProvider{}, got)
	})
}

func TestPopulateCloudProviderFromConfig(t *testing.T) {
	t.Run("parses AWS ARN", func(t *testing.T) {
		got, err := populateCloudProviderFromConfig(&CloudProvider{
			AWS: &AWSCloudProviderInfo{ARN: "arn:aws:rds:us-east-1:123456789012:db:products-db"},
		})
		require.NoError(t, err)
		require.NotNil(t, got.AWS)
		assert.Equal(t, "us-east-1", got.AWS.ARN.Region)
		assert.Equal(t, "123456789012", got.AWS.ARN.AccountID)
		assert.Equal(t, "db:products-db", got.AWS.ARN.Resource)
	})

	t.Run("errors on an invalid AWS ARN", func(t *testing.T) {
		_, err := populateCloudProviderFromConfig(&CloudProvider{
			AWS: &AWSCloudProviderInfo{ARN: "not-an-arn"},
		})
		require.Error(t, err)
	})

	t.Run("copies Azure details", func(t *testing.T) {
		got, err := populateCloudProviderFromConfig(&CloudProvider{
			Azure: &AzureCloudProviderInfo{
				SubscriptionID: "sub-12345-abcde",
				ResourceGroup:  "my-resource-group",
				ServerName:     "products-db",
			},
		})
		require.NoError(t, err)

		assert.Equal(t, &database_observability.AzureCloudProviderInfo{
			SubscriptionID: "sub-12345-abcde",
			ResourceGroup:  "my-resource-group",
			ServerName:     "products-db",
		}, got.Azure)
	})

	t.Run("parses GCP connection name", func(t *testing.T) {
		got, err := populateCloudProviderFromConfig(&CloudProvider{
			GCP: &GCPCloudProviderInfo{ConnectionName: "my-project:us-central1:my-db"},
		})
		require.NoError(t, err)

		assert.Equal(t, &database_observability.GCPCloudProviderInfo{
			ProjectID:  "my-project",
			Region:     "us-central1",
			InstanceID: "my-db",
		}, got.GCP)
	})

	t.Run("errors on an invalid GCP connection name", func(t *testing.T) {
		_, err := populateCloudProviderFromConfig(&CloudProvider{
			GCP: &GCPCloudProviderInfo{ConnectionName: "invalid"},
		})
		require.Error(t, err)
	})
}
