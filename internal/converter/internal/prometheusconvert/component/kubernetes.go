package component

import (
	prom_kubernetes "github.com/prometheus/prometheus/discovery/kubernetes"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/discovery/kubernetes"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
)

func appendDiscoveryKubernetes(pb *build.PrometheusBlocks, label string, sdConfig *prom_kubernetes.SDConfig) discovery.Exports {
	discoveryKubernetesArgs := toDiscoveryKubernetes(sdConfig)
	name := []string{"discovery", "kubernetes"}
	block := common.NewBlockWithOverride(name, label, discoveryKubernetesArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.kubernetes." + label + ".targets")
}

func ValidateDiscoveryKubernetes(sdConfig *prom_kubernetes.SDConfig) diag.Diagnostics {
	return common.ValidateHttpClientConfig(&sdConfig.HTTPClientConfig)
}

func toDiscoveryKubernetes(sdConfig *prom_kubernetes.SDConfig) *kubernetes.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &kubernetes.Arguments{
		APIServer:          config.URL(sdConfig.APIServer),
		Role:               string(sdConfig.Role),
		KubeConfig:         sdConfig.KubeConfig,
		HTTPClientConfig:   *common.ToHttpClientConfig(&sdConfig.HTTPClientConfig),
		NamespaceDiscovery: toNamespaceDiscovery(&sdConfig.NamespaceDiscovery),
		Selectors:          toSelectorConfig(sdConfig.Selectors),
		AttachMetadata:     toAttachMetadata(&sdConfig.AttachMetadata),
	}
}

func toNamespaceDiscovery(ndConfig *prom_kubernetes.NamespaceDiscovery) kubernetes.NamespaceDiscovery {
	return kubernetes.NamespaceDiscovery{
		IncludeOwnNamespace: ndConfig.IncludeOwnNamespace,
		Names:               ndConfig.Names,
	}
}

func toSelectorConfig(selectors []prom_kubernetes.SelectorConfig) []kubernetes.SelectorConfig {
	var result []kubernetes.SelectorConfig

	for _, selector := range selectors {
		result = append(result, kubernetes.SelectorConfig{
			Role:  string(selector.Role),
			Label: selector.Label,
			Field: selector.Field,
		})
	}

	return result
}

func toAttachMetadata(amConfig *prom_kubernetes.AttachMetadataConfig) kubernetes.AttachMetadataConfig {
	return kubernetes.AttachMetadataConfig{
		Node:      amConfig.Node,
		Namespace: amConfig.Namespace,
	}
}
