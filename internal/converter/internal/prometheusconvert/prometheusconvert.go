package prometheusconvert

import (
	"bytes"
	"fmt"
	"log/slog"

	prom_config "github.com/prometheus/prometheus/config"
	prom_discover "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/component"

	_ "github.com/prometheus/prometheus/discovery/install" // Register Prometheus SDs

	"github.com/grafana/alloy/syntax/token/builder"
)

// Convert implements a Prometheus config converter.
//
// extraArgs are supported to mirror the other converter params due to shared
// testing code but they should be passed empty to this converter.
func Convert(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(extraArgs) > 0 {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("extra arguments are not supported for the prometheus converter: %s", extraArgs))
		return nil, diags
	}

	promConfig, err := prom_config.Load(string(in), slog.New(slog.DiscardHandler))
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to parse Prometheus config: %s", err))
		return nil, diags
	}

	f := builder.NewFile()
	diags = AppendAll(f, promConfig)
	diags.AddAll(common.ValidateNodes(f))

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to render Alloy config: %s", err.Error()))
		return nil, diags
	}

	if len(buf.Bytes()) == 0 {
		return nil, diags
	}

	prettyByte, newDiags := common.PrettyPrint(buf.Bytes())
	diags.AddAll(newDiags)
	return prettyByte, diags
}

// AppendAll analyzes the entire prometheus config in memory and transforms it
// into Alloy component Arguments. It then appends each argument to the file
// builder. Exports from other components are correctly referenced to build the
// Alloy pipeline.
func AppendAll(f *builder.File, promConfig *prom_config.Config) diag.Diagnostics {
	return AppendAllNested(f, promConfig, nil, []discovery.Target{}, nil)
}

// AppendAllNested analyzes the entire prometheus config in memory and transforms it
// into Alloy component Arguments. It then appends each argument to the file builder.
// Exports from other components are correctly referenced to build the Alloy
// pipeline. Additional options can be provided overriding the job name, extra
// scrape targets, and predefined remote write exports.
func AppendAllNested(f *builder.File, promConfig *prom_config.Config, jobNameToCompLabelsFunc func(string) string, extraScrapeTargets []discovery.Target, remoteWriteExports *remotewrite.Exports) diag.Diagnostics {
	pb := build.NewPrometheusBlocks()

	if remoteWriteExports == nil {
		labelPrefix := ""
		if jobNameToCompLabelsFunc != nil {
			labelPrefix = jobNameToCompLabelsFunc("")
			if labelPrefix != "" {
				labelPrefix = common.SanitizeIdentifierPanics(labelPrefix)
			}
		}
		remoteWriteExports = component.AppendPrometheusRemoteWrite(pb, promConfig.GlobalConfig, promConfig.RemoteWriteConfigs, labelPrefix)
	}
	remoteWriteForwardTo := []storage.Appendable{remoteWriteExports.Receiver}

	for _, scrapeConfig := range promConfig.ScrapeConfigs {
		scrapeForwardTo := remoteWriteForwardTo
		label := scrapeConfig.JobName
		if jobNameToCompLabelsFunc != nil {
			label = jobNameToCompLabelsFunc(scrapeConfig.JobName)
		}
		label = common.SanitizeIdentifierPanics(label)

		promMetricsRelabelExports := component.AppendPrometheusRelabel(pb, scrapeConfig.MetricRelabelConfigs, remoteWriteForwardTo, label)
		if promMetricsRelabelExports != nil {
			scrapeForwardTo = []storage.Appendable{promMetricsRelabelExports.Receiver}
		}

		scrapeTargets := AppendServiceDiscoveryConfigs(pb, scrapeConfig.ServiceDiscoveryConfigs, label)
		scrapeTargets = append(scrapeTargets, extraScrapeTargets...)

		promDiscoveryRelabelExports := component.AppendDiscoveryRelabel(pb, scrapeConfig.RelabelConfigs, scrapeTargets, label)
		if promDiscoveryRelabelExports != nil {
			scrapeTargets = promDiscoveryRelabelExports.Output
		}

		component.AppendPrometheusScrape(pb, scrapeConfig, scrapeForwardTo, scrapeTargets, label)
	}

	diags := validate(promConfig)
	diags.AddAll(pb.GetScrapeInfo())

	pb.AppendToBody(f.Body())

	return diags
}

// AppendServiceDiscoveryConfigs will loop through the service discovery
// configs and append them to the file. This returns the scrape targets
// and discovery targets as a result.
func AppendServiceDiscoveryConfigs(pb *build.PrometheusBlocks, serviceDiscoveryConfig prom_discover.Configs, label string) []discovery.Target {
	var targets []discovery.Target
	labelCounts := make(map[string]int)
	for _, serviceDiscoveryConfig := range serviceDiscoveryConfig {
		exports := component.AppendServiceDiscoveryConfig(pb, serviceDiscoveryConfig, label, labelCounts)
		targets = append(targets, exports.Targets...)
	}

	return targets
}
