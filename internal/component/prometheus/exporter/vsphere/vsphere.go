package vsphere

import (
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/vmware_exporter"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.vsphere",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "vsphere"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the vsphere exporter
var DefaultArguments = Arguments{
	ChunkSize:               256,
	CollectConcurrency:      8,
	ObjectDiscoveryInterval: 0,
	EnableExporterMetrics:   true,
}

// Arguments controls the vsphere exporter.
type Arguments struct {
	ChunkSize               int               `river:"request_chunk_size,attr,optional"`
	CollectConcurrency      int               `river:"collect_concurrency,attr,optional"`
	VSphereURL              string            `river:"vsphere_url,attr,optional"`
	VSphereUser             string            `river:"vsphere_user,attr,optional"`
	VSpherePass             rivertypes.Secret `river:"vsphere_password,attr,optional"`
	ObjectDiscoveryInterval time.Duration     `river:"discovery_interval,attr,optional"`
	EnableExporterMetrics   bool              `river:"enable_exporter_metrics,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *vmware_exporter.Config {
	return &vmware_exporter.Config{
		ChunkSize:               a.ChunkSize,
		CollectConcurrency:      a.CollectConcurrency,
		VSphereURL:              a.VSphereURL,
		VSphereUser:             a.VSphereUser,
		VSpherePass:             config_util.Secret(a.VSpherePass),
		ObjectDiscoveryInterval: a.ObjectDiscoveryInterval,
		EnableExporterMetrics:   a.EnableExporterMetrics,
	}
}
