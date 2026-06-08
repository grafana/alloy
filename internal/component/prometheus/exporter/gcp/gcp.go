package gcp

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/gcp_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.gcp",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "gcp"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
	ProjectIDs            []string      `alloy:"project_ids,attr"`
	MetricPrefixes        []string      `alloy:"metrics_prefixes,attr"`
	ExtraFilters          []string      `alloy:"extra_filters,attr,optional"`
	RequestInterval       time.Duration `alloy:"request_interval,attr,optional"`
	RequestOffset         time.Duration `alloy:"request_offset,attr,optional"`
	IngestDelay           bool          `alloy:"ingest_delay,attr,optional"`
	DropDelegatedProjects bool          `alloy:"drop_delegated_projects,attr,optional"`
	ClientTimeout         time.Duration `alloy:"gcp_client_timeout,attr,optional"`
}

var DefaultArguments = Arguments{
	ClientTimeout:         15 * time.Second,
	RequestInterval:       5 * time.Minute,
	RequestOffset:         0,
	IngestDelay:           false,
	DropDelegatedProjects: false,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
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
