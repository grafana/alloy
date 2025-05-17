package process

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/common"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/process_exporter"
	exporter_config "github.com/ncabatoff/process-exporter/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.process",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createIntegration, "process"),
	})
}

func createIntegration(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := common.HostNameInstanceKey() // if cannot resolve instance key, use the host name for process exporter
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
	ProcessExporter []MatcherGroup `alloy:"matcher,block,optional"`

	ProcFSPath string `alloy:"procfs_path,attr,optional"`
	Children   bool   `alloy:"track_children,attr,optional"`
	Threads    bool   `alloy:"track_threads,attr,optional"`
	SMaps      bool   `alloy:"gather_smaps,attr,optional"`
	Recheck    bool   `alloy:"recheck_on_scrape,attr,optional"`
}

// MatcherGroup taken and converted to Alloy from github.com/ncabatoff/process-exporter/config
type MatcherGroup struct {
	Name         string   `alloy:"name,attr,optional"`
	CommRules    []string `alloy:"comm,attr,optional"`
	ExeRules     []string `alloy:"exe,attr,optional"`
	CmdlineRules []string `alloy:"cmdline,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
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
