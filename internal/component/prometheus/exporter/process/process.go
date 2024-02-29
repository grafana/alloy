package process

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/process_exporter"
	exporter_config "github.com/ncabatoff/process-exporter/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.process",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createIntegration, "process"),
	})
}

func createIntegration(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default arguments for the prometheus.exporter.process
// component.
var DefaultArguments = Arguments{
	ProcFSPath: "/proc",
	Children:   true,
	Threads:    true,
	SMaps:      true,
	Recheck:    false,
}

// Arguments configures the prometheus.exporter.process component
type Arguments struct {
	ProcessExporter []MatcherGroup `river:"matcher,block,optional"`

	ProcFSPath string `river:"procfs_path,attr,optional"`
	Children   bool   `river:"track_children,attr,optional"`
	Threads    bool   `river:"track_threads,attr,optional"`
	SMaps      bool   `river:"gather_smaps,attr,optional"`
	Recheck    bool   `river:"recheck_on_scrape,attr,optional"`
}

// MatcherGroup taken and converted to River from github.com/ncabatoff/process-exporter/config
type MatcherGroup struct {
	Name         string   `river:"name,attr,optional"`
	CommRules    []string `river:"comm,attr,optional"`
	ExeRules     []string `river:"exe,attr,optional"`
	CmdlineRules []string `river:"cmdline,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *process_exporter.Config {
	return &process_exporter.Config{
		ProcessExporter: convertMatcherGroups(a.ProcessExporter),
		ProcFSPath:      a.ProcFSPath,
		Children:        a.Children,
		Threads:         a.Threads,
		SMaps:           a.SMaps,
		Recheck:         a.Recheck,
	}
}

func convertMatcherGroups(m []MatcherGroup) exporter_config.MatcherRules {
	var out exporter_config.MatcherRules
	for _, v := range m {
		out = append(out, exporter_config.MatcherGroup(v))
	}
	return out
}
