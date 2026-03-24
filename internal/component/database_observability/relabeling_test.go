package database_observability

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/stretchr/testify/require"
)

func Test_GetRelabelingRules(t *testing.T) {
	t.Run("return relabeling rules", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", nil)

		require.Equal(t, 1, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)
	})

	t.Run("return relabeling rules with AWS config", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", &CloudProvider{
			AWS: &AWSCloudProviderInfo{
				ARN: arn.ARN{
					Region:    "some-region",
					AccountID: "some-account",
				},
			},
		})

		require.Equal(t, 4, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)

		require.Equal(t, "aws", rr[1].Replacement)
		require.Equal(t, "provider_name", rr[1].TargetLabel)
		require.Equal(t, relabel.Replace, rr[1].Action)

		require.Equal(t, "some-region", rr[2].Replacement)
		require.Equal(t, "provider_region", rr[2].TargetLabel)
		require.Equal(t, relabel.Replace, rr[2].Action)

		require.Equal(t, "some-account", rr[3].Replacement)
		require.Equal(t, "provider_account", rr[3].TargetLabel)
		require.Equal(t, relabel.Replace, rr[3].Action)
	})

	t.Run("return relabeling rules with Azure config", func(t *testing.T) {
		rr := GetRelabelingRules("some-server-id", &CloudProvider{
			Azure: &AzureCloudProviderInfo{
				ServerName:     "some-resource",
				ResourceGroup:  "some-resource-group",
				SubscriptionID: "some-subscription-id",
			},
		})

		require.Equal(t, 4, len(rr))
		require.Equal(t, "some-server-id", rr[0].Replacement)
		require.Equal(t, "server_id", rr[0].TargetLabel)
		require.Equal(t, relabel.Replace, rr[0].Action)

		require.Equal(t, "azure", rr[1].Replacement)
		require.Equal(t, "provider_name", rr[1].TargetLabel)
		require.Equal(t, relabel.Replace, rr[1].Action)

		require.Equal(t, "some-resource-group", rr[2].Replacement)
		require.Equal(t, "provider_region", rr[2].TargetLabel)
		require.Equal(t, relabel.Replace, rr[2].Action)

		require.Equal(t, "some-subscription-id", rr[3].Replacement)
		require.Equal(t, "provider_account", rr[3].TargetLabel)
		require.Equal(t, relabel.Replace, rr[3].Action)
	})
}
