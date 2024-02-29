package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/nerve"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_nerve "github.com/prometheus/prometheus/discovery/zookeeper"
)

func appendDiscoveryNerve(pb *build.PrometheusBlocks, label string, sdConfig *prom_nerve.NerveSDConfig) discovery.Exports {
	discoveryNerveArgs := toDiscoveryNerve(sdConfig)
	name := []string{"discovery", "nerve"}
	block := common.NewBlockWithOverride(name, label, discoveryNerveArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.nerve." + label + ".targets")
}

func ValidateDiscoveryNerve(sdConfig *prom_nerve.NerveSDConfig) diag.Diagnostics {
	return nil
}

func toDiscoveryNerve(sdConfig *prom_nerve.NerveSDConfig) *nerve.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &nerve.Arguments{
		Servers: sdConfig.Servers,
		Paths:   sdConfig.Paths,
		Timeout: time.Duration(sdConfig.Timeout),
	}
}
