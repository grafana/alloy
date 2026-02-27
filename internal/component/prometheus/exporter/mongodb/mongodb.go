package mongodb

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.mongodb",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "mongodb"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
	URI                      alloytypes.Secret `alloy:"mongodb_uri,attr"`
	LogLevel                 string            `alloy:"log_level,attr,optional"`
	CompatibleMode           bool              `alloy:"compatible_mode,attr,optional"`
	CollectAll               bool              `alloy:"collect_all,attr,optional"`
	DirectConnect            bool              `alloy:"direct_connect,attr,optional"`
	DiscoveringMode          bool              `alloy:"discovering_mode,attr,optional"`
	EnableDBStats            bool              `alloy:"enable_db_stats,attr,optional"`
	EnableDBStatsFreeStorage bool              `alloy:"enable_db_stats_free_storage,attr,optional"`
	EnableDiagnosticData     bool              `alloy:"enable_diagnostic_data,attr,optional"`
	EnableReplicasetStatus   bool              `alloy:"enable_replicaset_status,attr,optional"`
	EnableReplicasetConfig   bool              `alloy:"enable_replicaset_config,attr,optional"`
	EnableCurrentopMetrics   bool              `alloy:"enable_currentop_metrics,attr,optional"`
	EnableTopMetrics         bool              `alloy:"enable_top_metrics,attr,optional"`
	EnableIndexStats         bool              `alloy:"enable_index_stats,attr,optional"`
	EnableCollStats          bool              `alloy:"enable_coll_stats,attr,optional"`
	EnableProfile            bool              `alloy:"enable_profile,attr,optional"`
	EnableShards             bool              `alloy:"enable_shards,attr,optional"`
	EnableFCV                bool              `alloy:"enable_fcv,attr,optional"`
	EnablePBMMetrics         bool              `alloy:"enable_pbm_metrics,attr,optional"`
}

func (a *Arguments) Convert() *mongodb_exporter.Config {
	cfg := &mongodb_exporter.Config{
		URI:                      config_util.Secret(a.URI),
		CompatibleMode:           a.CompatibleMode,
		CollectAll:               a.CollectAll,
		DirectConnect:            a.DirectConnect,
		DiscoveringMode:          a.DiscoveringMode,
		EnableDBStats:            a.EnableDBStats,
		EnableDBStatsFreeStorage: a.EnableDBStatsFreeStorage,
		EnableDiagnosticData:     a.EnableDiagnosticData,
		EnableReplicasetStatus:   a.EnableReplicasetStatus,
		EnableReplicasetConfig:   a.EnableReplicasetConfig,
		EnableCurrentopMetrics:   a.EnableCurrentopMetrics,
		EnableTopMetrics:         a.EnableTopMetrics,
		EnableIndexStats:         a.EnableIndexStats,
		EnableCollStats:          a.EnableCollStats,
		EnableProfile:            a.EnableProfile,
		EnableShards:             a.EnableShards,
		EnableFCV:                a.EnableFCV,
		EnablePBMMetrics:         a.EnablePBMMetrics,
	}
	if a.LogLevel != "" {
		_ = cfg.LogLevel.Set(a.LogLevel)
	}
	return cfg
}

// SetToDefault sets the default values for the Arguments.
func (a *Arguments) SetToDefault() {
	a.LogLevel = "info"
	a.DirectConnect = false
	a.CompatibleMode = true
	a.CollectAll = true
	a.DiscoveringMode = false
	a.EnableDBStats = false
	a.EnableDBStatsFreeStorage = false
	a.EnableDiagnosticData = false
	a.EnableReplicasetStatus = false
	a.EnableReplicasetConfig = false
	a.EnableCurrentopMetrics = false
	a.EnableTopMetrics = false
	a.EnableIndexStats = false
	a.EnableCollStats = false
	a.EnableProfile = false
	a.EnableShards = false
	a.EnableFCV = false
	a.EnablePBMMetrics = false
}
