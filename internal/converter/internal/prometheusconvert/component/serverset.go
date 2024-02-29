package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/serverset"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_zk "github.com/prometheus/prometheus/discovery/zookeeper"
)

func appendDiscoveryServerset(pb *build.PrometheusBlocks, label string, sdc *prom_zk.ServersetSDConfig) discovery.Exports {
	discoveryServersetArgs := toDiscoveryServerset(sdc)
	name := []string{"discovery", "serverset"}
	block := common.NewBlockWithOverride(name, label, discoveryServersetArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.serverset." + label + ".targets")
}

func toDiscoveryServerset(sdc *prom_zk.ServersetSDConfig) *serverset.Arguments {
	if sdc == nil {
		return nil
	}

	return &serverset.Arguments{
		Servers: sdc.Servers,
		Paths:   sdc.Paths,
		Timeout: time.Duration(sdc.Timeout),
	}
}

func ValidateDiscoveryServerset(_ *prom_zk.ServersetSDConfig) diag.Diagnostics {
	return nil
}
