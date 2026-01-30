package database_observability

import "github.com/grafana/alloy/internal/component/common/relabel"

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
			providerName.TargetLabel = "provider_name"
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.AWS.ARN.Region
			providerRegion.TargetLabel = "provider_region"
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.AWS.ARN.AccountID
			providerAccount.TargetLabel = "provider_account"
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
		if cp.Azure != nil {
			providerName := relabel.DefaultRelabelConfig
			providerName.Replacement = "azure"
			providerName.TargetLabel = "provider_name"
			providerName.Action = relabel.Replace

			providerRegion := relabel.DefaultRelabelConfig
			providerRegion.Replacement = cp.Azure.ResourceGroup
			providerRegion.TargetLabel = "provider_region"
			providerRegion.Action = relabel.Replace

			providerAccount := relabel.DefaultRelabelConfig
			providerAccount.Replacement = cp.Azure.SubscriptionID
			providerAccount.TargetLabel = "provider_account"
			providerAccount.Action = relabel.Replace

			rs = append(rs, &providerName, &providerRegion, &providerAccount)
		}
	}

	return rs
}
