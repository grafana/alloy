package database_observability

import "github.com/grafana/alloy/internal/component/common/relabel"

const (
	ProviderNameLabel    = "provider_name"
	ProviderRegionLabel  = "provider_region"
	ProviderAccountLabel = "provider_account"
)

func GetRelabelingRules(serverID string, cp *CloudProvider) []*relabel.Config {
	r := relabel.DefaultRelabelConfig // use default to avoid defining all fields
	r.Replacement = serverID
	r.TargetLabel = "server_id"
	r.Action = relabel.Replace

	rs := []*relabel.Config{&r}

	if cp != nil {
		if cp.AWS != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "aws"
			providerName.TargetLabel = ProviderNameLabel
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.AWS.ARN.Region
			providerRegion.TargetLabel = ProviderRegionLabel
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.AWS.ARN.AccountID
			providerAccount.TargetLabel = ProviderAccountLabel
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
		if cp.Azure != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "azure"
			providerName.TargetLabel = ProviderNameLabel
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.Azure.ResourceGroup
			providerRegion.TargetLabel = ProviderRegionLabel
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.Azure.SubscriptionID
			providerAccount.TargetLabel = ProviderAccountLabel
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
		if cp.GCP != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "gcp"
			providerName.TargetLabel = ProviderNameLabel
			providerName.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.GCP.ProjectID
			providerAccount.TargetLabel = ProviderAccountLabel
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerAccount)
		}
	}

	return rs
}
