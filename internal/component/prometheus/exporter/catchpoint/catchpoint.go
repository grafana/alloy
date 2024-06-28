package catchpoint

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/catchpoint_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.catchpoint",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   exporter.Exports{},
		Build:     exporter.New(createExporter, "catchpoint"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the catchpoint exporter
var DefaultArguments = Arguments{
	VerboseLogging: false,
	WebhookPath:    "/catchpoint-webhook",
	Port:           "9090",
}

// Arguments controls the catchpoint exporter.
type Arguments struct {
	VerboseLogging bool   `alloy:"verbose_logging,attr,optional"`
	WebhookPath    string `alloy:"webhook_path,attr,optional"`
	Port           string `alloy:"port,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *catchpoint_exporter.Config {
	return &catchpoint_exporter.Config{
		VerboseLogging: a.VerboseLogging,
		WebhookPath:    a.WebhookPath,
		Port:           a.Port,
	}
}
