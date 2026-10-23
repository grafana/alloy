package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCloudProvider(t *testing.T) {
	require.NoError(t, validateCloudProvider(nil))
	require.NoError(t, validateCloudProvider(&CloudProvider{GCP: &GCPCloudProviderInfo{ConnectionName: "p:r:i"}}))

	err := validateCloudProvider(&CloudProvider{
		AWS:   &AWSCloudProviderInfo{ARN: "arn:aws:rds:us-east-1:123456789012:db:mydb"},
		Azure: &AzureCloudProviderInfo{SubscriptionID: "sub", ResourceGroup: "rg"},
	})
	assert.EqualError(t, err, "cloud_provider: at most one of aws, azure, or gcp must be specified")
}
