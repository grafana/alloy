package metricsutils

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/alloy/internal/static/integrations/v2/common"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
)

// NewMetricsHandlerIntegration returns a integrations.MetricsIntegration which
// will expose a /metrics endpoint for h.
func NewMetricsHandlerIntegration(
	_ log.Logger,
	c integrations.Config,
	mc common.MetricsConfig,
	globals integrations.Globals,
	h http.Handler,
) (integrations.MetricsIntegration, error) {

	id, err := c.Identifier(globals)
	if err != nil {
		return nil, err
	}
	return &metricsHandlerIntegration{
		integrationName: c.Name(),
		instanceID:      id,

		common:  mc,
		globals: globals,
		handler: h,

		targets: []handlerTarget{{MetricsPath: "metrics"}},
	}, nil
}

type metricsHandlerIntegration struct {
	integrationName, instanceID string

	common  common.MetricsConfig
	globals integrations.Globals
	handler http.Handler
	targets []handlerTarget

	runFunc func(ctx context.Context) error
}

type handlerTarget struct {
	// Path relative to Handler prefix where metrics are available.
	MetricsPath string
	// Extra labels to inject into the target. Labels here that take precedence
	// over labels with the same name from the generated target group.
	Labels model.LabelSet
}

// Static typecheck tests
var (
	_ integrations.Integration        = (*metricsHandlerIntegration)(nil)
	_ integrations.HTTPIntegration    = (*metricsHandlerIntegration)(nil)
	_ integrations.MetricsIntegration = (*metricsHandlerIntegration)(nil)
)

// RunIntegration implements Integration.
func (i *metricsHandlerIntegration) RunIntegration(ctx context.Context) error {
	// Call our runFunc if defined (used from integrationShim), otherwise
	// fallback to no-op.
	if i.runFunc != nil {
		return i.runFunc(ctx)
	}

	<-ctx.Done()
	return nil
}

// Handler implements HTTPIntegration.
func (i *metricsHandlerIntegration) Handler(prefix string) (http.Handler, error) {
	r := mux.NewRouter()
	r.Handle(path.Join(prefix, "metrics"), i.handler)
	return r, nil
}

// Targets implements MetricsIntegration.
func (i *metricsHandlerIntegration) Targets(ep integrations.Endpoint) []*targetgroup.Group {
	integrationNameValue := model.LabelValue("integrations/" + i.integrationName)

	group := &targetgroup.Group{
		Labels: model.LabelSet{
			model.InstanceLabel: model.LabelValue(i.instanceID),
			model.JobLabel:      integrationNameValue,
			"agent_hostname":    model.LabelValue(i.globals.AgentIdentifier),

			// Meta labels that can be used during SD.
			"__meta_agent_integration_name":       model.LabelValue(i.integrationName),
			"__meta_agent_integration_instance":   model.LabelValue(i.instanceID),
			"__meta_agent_integration_autoscrape": model.LabelValue(BoolToString(*i.common.Autoscrape.Enable)),
		},
		Source: fmt.Sprintf("%s/%s", i.integrationName, i.instanceID),
	}

	i.common.ExtraLabels.Range(func(lbl labels.Label) {
		group.Labels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	for _, t := range i.targets {
		group.Targets = append(group.Targets, model.LabelSet{
			model.AddressLabel:     model.LabelValue(ep.Host),
			model.MetricsPathLabel: model.LabelValue(path.Join(ep.Prefix, t.MetricsPath)),
		}.Merge(t.Labels))
	}

	return []*targetgroup.Group{group}
}

// BoolToString is a helper for converting boolean values to a Prometheus labels-compatible string.
func BoolToString(b bool) string {
	switch b {
	case true:
		return "1"
	default:
		return "0"
	}
}

// ScrapeConfigs implements MetricsIntegration.
func (i *metricsHandlerIntegration) ScrapeConfigs(sd discovery.Configs) []*autoscrape.ScrapeConfig {
	if !*i.common.Autoscrape.Enable {
		return nil
	}

	cfg := config.DefaultScrapeConfig
	cfg.JobName = fmt.Sprintf("%s/%s", i.integrationName, i.instanceID)
	cfg.Scheme = i.globals.AgentBaseURL.Scheme
	cfg.ServiceDiscoveryConfigs = sd
	cfg.ScrapeInterval = i.common.Autoscrape.ScrapeInterval
	cfg.ScrapeTimeout = i.common.Autoscrape.ScrapeTimeout
	cfg.RelabelConfigs = i.common.Autoscrape.RelabelConfigs
	cfg.MetricRelabelConfigs = i.common.Autoscrape.MetricRelabelConfigs

	return []*autoscrape.ScrapeConfig{{
		Instance: i.common.Autoscrape.MetricsInstance,
		Config:   cfg,
	}}
}
