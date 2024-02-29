package component

import (
	"time"

	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/discovery/file"
	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/agent/internal/converter/internal/prometheusconvert/build"
	prom_file "github.com/prometheus/prometheus/discovery/file"
)

func appendDiscoveryFile(pb *build.PrometheusBlocks, label string, sdConfig *prom_file.SDConfig) discovery.Exports {
	discoveryFileArgs := toDiscoveryFile(sdConfig)
	name := []string{"discovery", "file"}
	block := common.NewBlockWithOverride(name, label, discoveryFileArgs)
	pb.DiscoveryBlocks = append(pb.DiscoveryBlocks, build.NewPrometheusBlock(block, name, label, "", ""))
	return common.NewDiscoveryExports("discovery.file." + label + ".targets")
}

func ValidateDiscoveryFile(sdConfig *prom_file.SDConfig) diag.Diagnostics {
	return make(diag.Diagnostics, 0)
}

func toDiscoveryFile(sdConfig *prom_file.SDConfig) *file.Arguments {
	if sdConfig == nil {
		return nil
	}

	return &file.Arguments{
		Files:           sdConfig.Files,
		RefreshInterval: time.Duration(sdConfig.RefreshInterval),
	}
}
