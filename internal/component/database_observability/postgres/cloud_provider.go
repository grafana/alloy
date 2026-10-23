package postgres

import (
	"fmt"

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
	if cp == nil {
		return nil
	}
	count := 0
	if cp.AWS != nil {
		count++
	}
	if cp.Azure != nil {
		count++
	}
	if cp.GCP != nil {
		count++
	}
	if count > 1 {
		return fmt.Errorf("cloud_provider: at most one of aws, azure, or gcp must be specified")
	}
	return nil
}
