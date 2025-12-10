package databricks

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/static/integrations"
	"github.com/grafana/alloy/internal/static/integrations/databricks_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.exporter.databricks",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   exporter.Exports{},

		Build: exporter.New(createExporter, "databricks"),
	})
}

func createExporter(opts component.Options, args component.Arguments) (integrations.Integration, string, error) {
	a := args.(Arguments)
	defaultInstanceKey := opts.ID // if cannot resolve instance key, use the component ID
	return integrations.NewIntegrationWithInstanceKey(opts.Logger, a.Convert(), defaultInstanceKey)
}

// DefaultArguments holds the default settings for the databricks exporter
var DefaultArguments = Arguments{
	QueryTimeout:        5 * time.Minute,
	BillingLookback:     24 * time.Hour,
	JobsLookback:        2 * time.Hour,
	PipelinesLookback:   2 * time.Hour,
	QueriesLookback:     1 * time.Hour,
	SLAThresholdSeconds: 3600,
	CollectTaskRetries:  false,
}

// Arguments controls the databricks exporter.
type Arguments struct {
	ServerHostname      string            `alloy:"server_hostname,attr"`
	WarehouseHTTPPath   string            `alloy:"warehouse_http_path,attr"`
	ClientID            string            `alloy:"client_id,attr"`
	ClientSecret        alloytypes.Secret `alloy:"client_secret,attr"`
	QueryTimeout        time.Duration     `alloy:"query_timeout,attr,optional"`
	BillingLookback     time.Duration     `alloy:"billing_lookback,attr,optional"`
	JobsLookback        time.Duration     `alloy:"jobs_lookback,attr,optional"`
	PipelinesLookback   time.Duration     `alloy:"pipelines_lookback,attr,optional"`
	QueriesLookback     time.Duration     `alloy:"queries_lookback,attr,optional"`
	SLAThresholdSeconds int               `alloy:"sla_threshold_seconds,attr,optional"`
	CollectTaskRetries  bool              `alloy:"collect_task_retries,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

func (a *Arguments) Convert() *databricks_exporter.Config {
	return &databricks_exporter.Config{
		ServerHostname:      a.ServerHostname,
		WarehouseHTTPPath:   a.WarehouseHTTPPath,
		ClientID:            a.ClientID,
		ClientSecret:        config_util.Secret(a.ClientSecret),
		QueryTimeout:        a.QueryTimeout,
		BillingLookback:     a.BillingLookback,
		JobsLookback:        a.JobsLookback,
		PipelinesLookback:   a.PipelinesLookback,
		QueriesLookback:     a.QueriesLookback,
		SLAThresholdSeconds: a.SLAThresholdSeconds,
		CollectTaskRetries:  a.CollectTaskRetries,
	}
}

