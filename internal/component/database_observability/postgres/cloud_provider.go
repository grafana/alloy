package postgres

import (
	"github.com/grafana/alloy/internal/component/database_observability/postgres/dbinstance"
)

// The cloud_provider block is defined in dbinstance, which resolves it at connect time.
type (
	CloudProvider          = dbinstance.CloudProvider
	AWSCloudProviderInfo   = dbinstance.AWSCloudProviderInfo
	AzureCloudProviderInfo = dbinstance.AzureCloudProviderInfo
	GCPCloudProviderInfo   = dbinstance.GCPCloudProviderInfo
)

func validateCloudProvider(cp *CloudProvider) error {
	return dbinstance.ValidateCloudProvider(cp)
}
