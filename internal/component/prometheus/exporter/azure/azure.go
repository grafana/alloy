package azure

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/azure_exporter"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.azure",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "azure"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

type Arguments struct {
	Subscriptions            []string `alloy:"subscriptions,attr"`
	ResourceGraphQueryFilter string   `alloy:"resource_graph_query_filter,attr,optional"`
	ResourceType             string   `alloy:"resource_type,attr"`
	Metrics                  []string `alloy:"metrics,attr"`
	MetricAggregations       []string `alloy:"metric_aggregations,attr,optional"`
	Timespan                 string   `alloy:"timespan,attr,optional"`
	IncludedDimensions       []string `alloy:"included_dimensions,attr,optional"`
	IncludedResourceTags     []string `alloy:"included_resource_tags,attr,optional"`
	MetricNamespace          string   `alloy:"metric_namespace,attr,optional"`
	MetricNameTemplate       string   `alloy:"metric_name_template,attr,optional"`
	MetricHelpTemplate       string   `alloy:"metric_help_template,attr,optional"`
	AzureCloudEnvironment    string   `alloy:"azure_cloud_environment,attr,optional"`
	ValidateDimensions       bool     `alloy:"validate_dimensions,attr,optional"`
	Regions                  []string `alloy:"regions,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Timespan:              "PT1M",
		MetricNameTemplate:    "azure_{type}_{metric}_{aggregation}_{unit}",
		MetricHelpTemplate:    "Azure metric {metric} for {type} with aggregation {aggregation} as {unit}",
		IncludedResourceTags:  []string{"owner"},
		AzureCloudEnvironment: "azurecloud",
		// Dimensions do not always apply to all metrics for a service, which requires you to configure multiple exporters
		//  to fully monitor a service which is tedious. Turning off validation eliminates this complexity. The underlying
		//  sdk will only give back the dimensions which are valid for particular metrics.
		ValidateDimensions: false,
	}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if err := a.Convert().Validate(); err != nil {
		return err
	}
	return nil
}

func (a *Arguments) Convert() *azure_exporter.Config {
	return &azure_exporter.Config{
		Subscriptions:            a.Subscriptions,
		ResourceGraphQueryFilter: a.ResourceGraphQueryFilter,
		ResourceType:             a.ResourceType,
		Metrics:                  a.Metrics,
		MetricAggregations:       a.MetricAggregations,
		Timespan:                 a.Timespan,
		IncludedDimensions:       a.IncludedDimensions,
		IncludedResourceTags:     a.IncludedResourceTags,
		MetricNamespace:          a.MetricNamespace,
		MetricNameTemplate:       a.MetricNameTemplate,
		MetricHelpTemplate:       a.MetricHelpTemplate,
		AzureCloudEnvironment:    a.AzureCloudEnvironment,
		ValidateDimensions:       a.ValidateDimensions,
		Regions:                  a.Regions,
	}
}
