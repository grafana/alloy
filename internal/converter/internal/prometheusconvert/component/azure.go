package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/azure"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/river/rivertypes"
	prom_azure "github.com/prometheus/prometheus/discovery/azure"
)

func appendDiscoveryAzure(pb *build.PrometheusBlocks, label string, sdConfig *prom_azure.SDConfig) discovery.Exports {
	discoveryAzureArgs := toDiscoveryAzure(sdConfig)
	name := []string{"discovery", "azure"}
	block := common.NewBlockWithOverride(name, label, discoveryAzureArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.azure." + label + ".targets")
}

func toDiscoveryAzure(sdConfig *prom_azure.SDConfig) *azure.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &azure.Arguments{
		Environment:     sdConfig.Environment,
		Port:            sdConfig.Port,
		SubscriptionID:  sdConfig.SubscriptionID,
		OAuth:           toDiscoveryAzureOauth2(sdConfig.ClientID, sdConfig.TenantID, string(sdConfig.ClientSecret)),
		ManagedIdentity: toManagedIdentity(sdConfig),
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
		ResourceGroup:   sdConfig.ResourceGroup,
		ProxyConfig:     common.ToProxyConfig(sdConfig.HTTPClientConfig.ProxyConfig),
		FollowRedirects: sdConfig.HTTPClientConfig.FollowRedirects,
		EnableHTTP2:     sdConfig.HTTPClientConfig.EnableHTTP2,
		TLSConfig:       *common.ToTLSConfig(&sdConfig.HTTPClientConfig.TLSConfig),
	}
}

func ValidateDiscoveryAzure(sdConfig *prom_azure.SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toManagedIdentity(sdConfig *prom_azure.SDConfig) *azure.ManagedIdentity {
	if sdConfig == nil {
		return nil
	}

	return &azure.ManagedIdentity{
		ClientID: sdConfig.ClientID,
	}
}

func toDiscoveryAzureOauth2(clientId string, tenantId string, clientSecret string) *azure.OAuth {
	return &azure.OAuth{
		ClientID:     clientId,
		TenantID:     tenantId,
		ClientSecret: rivertypes.Secret(clientSecret),
	}
}
