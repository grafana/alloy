package nvidiagpu

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/nvidiagpu_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.nvidiagpu",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "nvidiagpu"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds non-zero default options for Arguments when it is
// unmarshaled from Alloy.
var DefaultArguments = Arguments{
	NvidiaSmiCommand: "nvidia-smi",
}

type Arguments struct {
	NvidiaSmiCommand string `alloy:"nvidia_smi_command,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Convert converts the component arguments into the integration config.
func (a *Arguments) Convert() *nvidiagpu_exporter.Config {
	return &nvidiagpu_exporter.Config{
		NvidiaSmiCommand: a.NvidiaSmiCommand,
	}
}
