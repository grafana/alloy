package pyroscope

import (
	"github.com/grafana/alloy/internal/build"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	LabelOtelScopeName    = "otel.scope.name"
	LabelOtelScopeVersion = "otel.scope.version"

	ScopeNameScrape = "com.grafana.alloy/pyroscope.scrape"
	ScopeNameEBPF   = "com.grafana.alloy/pyroscope.ebpf"
	ScopeNameJava   = "com.grafana.alloy/pyroscope.java"
)

// AddScopeLabels fills missing instrumentation scope labels for profiles produced by Alloy.
func AddScopeLabels(builder *labels.Builder, scopeName string) {
	existingScopeName := builder.Get(LabelOtelScopeName)
	if existingScopeName == "" {
		builder.Set(LabelOtelScopeName, scopeName)
		existingScopeName = scopeName
	}
	if existingScopeName != scopeName {
		return
	}
	if version := ScopeVersion(); version != "" && builder.Get(LabelOtelScopeVersion) == "" {
		builder.Set(LabelOtelScopeVersion, version)
	}
}

// LabelsWithScope returns labels with instrumentation scope labels for profiles produced by Alloy.
func LabelsWithScope(lbls labels.Labels, scopeName string) labels.Labels {
	builder := labels.NewBuilder(lbls)
	AddScopeLabels(builder, scopeName)
	return builder.Labels()
}

// ScopeVersion returns the Alloy build version when it represents a real build.
func ScopeVersion() string {
	if build.Version == "" || build.Version == "v0.0.0" {
		return ""
	}
	return build.Version
}
