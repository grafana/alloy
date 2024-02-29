package gcp

import (
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/prometheus/exporter"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/static/integrations"
	"github.com/grafana/agent/internal/static/integrations/gcp_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.gcp",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "gcp"),
	})
}

func createExporter(opts component.Options, args component.Arguments, defaultInstanceKey string) (integrations.Integration, string, error) {
	a := args.(Arguments)
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
	ProjectIDs            []string      `river:"project_ids,attr"`
	MetricPrefixes        []string      `river:"metrics_prefixes,attr"`
	ExtraFilters          []string      `river:"extra_filters,attr,optional"`
	RequestInterval       time.Duration `river:"request_interval,attr,optional"`
	RequestOffset         time.Duration `river:"request_offset,attr,optional"`
	IngestDelay           bool          `river:"ingest_delay,attr,optional"`
	DropDelegatedProjects bool          `river:"drop_delegated_projects,attr,optional"`
	ClientTimeout         time.Duration `river:"gcp_client_timeout,attr,optional"`
}

var DefaultArguments = Arguments{
	ClientTimeout:         15 * time.Second,
	RequestInterval:       5 * time.Minute,
	RequestOffset:         0,
	IngestDelay:           false,
	DropDelegatedProjects: false,
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements river.Validator.
func (a *Arguments) Validate() error {
	if err := a.Convert().Validate(); err != nil {
		return err
	}
	return nil
}

func (a *Arguments) Convert() *gcp_exporter.Config {
	return &gcp_exporter.Config{
		ProjectIDs:            a.ProjectIDs,
		MetricPrefixes:        a.MetricPrefixes,
		ExtraFilters:          a.ExtraFilters,
		RequestInterval:       a.RequestInterval,
		RequestOffset:         a.RequestOffset,
		IngestDelay:           a.IngestDelay,
		DropDelegatedProjects: a.DropDelegatedProjects,
		ClientTimeout:         a.ClientTimeout,
	}
}
