package build

import (
	"fmt"
	"sort"

	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/relabel"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	prometheus_component "github.com/grafana/alloy/internal/converter/internal/prometheusconvert/component"
	"github.com/grafana/alloy/internal/static/traces/remotewriteexporter"
	prom_relabel "github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/storage"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, remoteWriteExporterConverter{})
}

type remoteWriteExporterConverter struct{}

func (remoteWriteExporterConverter) Factory() component.Factory {
	return remotewriteexporter.NewFactory()
}

func (remoteWriteExporterConverter) InputComponentName() string {
	return "otelcol.exporter.prometheus"
}

func (remoteWriteExporterConverter) ConvertAndAppend(state *otelcolconvert.State, id *componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	label := state.AlloyComponentLabel()

	// We overloaded the ServerConfig.Endpoint field to be the prometheus.remote_write label
	rwLabel := "metrics_" + cfg.(*remotewriteexporter.Config).PromInstance
	forwardTo := []storage.Appendable{common.ConvertAppendable{Expr: fmt.Sprintf("prometheus.remote_write.%s.receiver", rwLabel)}}
	if len(cfg.(*remotewriteexporter.Config).ConstLabels) > 0 {
		exports := includeRelabelConfig(label, cfg, state, forwardTo)
		forwardTo = []storage.Appendable{exports.Receiver}
	}

	args := toremotewriteexporterConfig(cfg.(*remotewriteexporter.Config), forwardTo)
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "prometheus"}, label, args)

	var diags diag.Diagnostics
	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", otelcolconvert.StringifyInstanceID(id), otelcolconvert.StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func includeRelabelConfig(label string, cfg component.Config, state *otelcolconvert.State, forwardTo []storage.Appendable) *relabel.Exports {
	pb := build.NewPrometheusBlocks()

	defaultRelabelConfigs := &alloy_relabel.Config{}
	defaultRelabelConfigs.SetToDefault()
	relabelConfigs := []*prom_relabel.Config{}

	// sort they keys for consistency in map iteration
	keys := make([]string, 0, len(cfg.(*remotewriteexporter.Config).ConstLabels))
	for label := range cfg.(*remotewriteexporter.Config).ConstLabels {
		keys = append(keys, label)
	}
	sort.Strings(keys)

	for _, label := range keys {
		relabelConfigs = append(relabelConfigs, &prom_relabel.Config{
			Separator:   defaultRelabelConfigs.Separator,
			Regex:       prom_relabel.Regexp(defaultRelabelConfigs.Regex),
			Modulus:     defaultRelabelConfigs.Modulus,
			TargetLabel: label,
			Replacement: cfg.(*remotewriteexporter.Config).ConstLabels[label],
			Action:      prom_relabel.Action(defaultRelabelConfigs.Action),
		})
	}

	exports := prometheus_component.AppendPrometheusRelabel(pb, relabelConfigs, forwardTo, label)
	pb.AppendToBody(state.Body())
	return exports
}

func toremotewriteexporterConfig(cfg *remotewriteexporter.Config, forwardTo []storage.Appendable) *prometheus.Arguments {
	defaultArgs := &prometheus.Arguments{}
	defaultArgs.SetToDefault()

	return &prometheus.Arguments{
		IncludeTargetInfo:             defaultArgs.IncludeTargetInfo,
		IncludeScopeInfo:              defaultArgs.IncludeScopeInfo,
		IncludeScopeLabels:            defaultArgs.IncludeScopeLabels,
		GCFrequency:                   cfg.StaleTime,
		ForwardTo:                     forwardTo,
		AddMetricSuffixes:             defaultArgs.AddMetricSuffixes,
		ResourceToTelemetryConversion: defaultArgs.ResourceToTelemetryConversion,
	}
}
