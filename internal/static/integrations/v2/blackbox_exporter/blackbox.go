package blackbox_exporter_v2

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/static/integrations/blackbox_exporter"
	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	blackbox_config "github.com/prometheus/blackbox_exporter/config"
	"github.com/prometheus/blackbox_exporter/prober"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
)

type blackboxHandler struct {
	cfg     *Config
	modules *blackbox_config.Config
	log     log.Logger
}

func (bbh *blackboxHandler) Targets(ep integrations.Endpoint) []*targetgroup.Group {
	integrationNameValue := model.LabelValue("integrations/" + bbh.cfg.Name())
	id, _ := bbh.cfg.Identifier(bbh.cfg.globals)

	group := &targetgroup.Group{
		Labels: model.LabelSet{
			model.InstanceLabel: model.LabelValue(id),
			model.JobLabel:      integrationNameValue,
			"agent_hostname":    model.LabelValue(bbh.cfg.globals.AgentIdentifier),

			// Meta labels that can be used during SD.
			"__meta_agent_integration_name":       model.LabelValue(bbh.cfg.Name()),
			"__meta_agent_integration_instance":   model.LabelValue(bbh.cfg.Name()),
			"__meta_agent_integration_autoscrape": model.LabelValue(metricsutils.BoolToString(*bbh.cfg.Common.Autoscrape.Enable)),
		},
		Source: fmt.Sprintf("%s/%s", bbh.cfg.Name(), bbh.cfg.Name()),
	}

	bbh.cfg.Common.ExtraLabels.Range(func(lbl labels.Label) {
		group.Labels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	for _, t := range bbh.cfg.BlackboxTargets {
		labelSet := model.LabelSet{
			model.AddressLabel:     model.LabelValue(ep.Host),
			model.MetricsPathLabel: model.LabelValue(path.Join(ep.Prefix, "metrics")),
			"blackbox_target":      model.LabelValue(t.Target),
			"__param_target":       model.LabelValue(t.Target),
		}

		if t.Module != "" {
			labelSet = labelSet.Merge(model.LabelSet{
				"__param_module": model.LabelValue(t.Module),
			})
		}
		group.Targets = append(group.Targets, labelSet)
	}

	return []*targetgroup.Group{group}
}

func (bbh *blackboxHandler) ScrapeConfigs(sd discovery.Configs) []*autoscrape.ScrapeConfig {
	if !*bbh.cfg.Common.Autoscrape.Enable {
		return nil
	}
	name := bbh.cfg.Name()
	cfg := config.DefaultScrapeConfig
	cfg.JobName = fmt.Sprintf("%s/%s", name, name)
	cfg.Scheme = bbh.cfg.globals.AgentBaseURL.Scheme
	cfg.ServiceDiscoveryConfigs = sd
	cfg.ScrapeInterval = bbh.cfg.Common.Autoscrape.ScrapeInterval
	cfg.ScrapeTimeout = bbh.cfg.Common.Autoscrape.ScrapeTimeout
	cfg.RelabelConfigs = bbh.cfg.Common.Autoscrape.RelabelConfigs
	cfg.MetricRelabelConfigs = bbh.cfg.Common.Autoscrape.MetricRelabelConfigs

	return []*autoscrape.ScrapeConfig{{
		Instance: bbh.cfg.Common.Autoscrape.MetricsInstance,
		Config:   cfg,
	}}
}

func (bbh *blackboxHandler) Handler(prefix string) (http.Handler, error) {
	r := mux.NewRouter()
	r.Handle(path.Join(prefix, "metrics"), bbh.createHandler(bbh.cfg.BlackboxTargets))

	return r, nil
}

func (bbh *blackboxHandler) createHandler(targets []blackbox_exporter.BlackboxTarget) http.HandlerFunc {
	blackboxTargets := make(map[string]blackbox_exporter.BlackboxTarget)
	for _, target := range targets {
		blackboxTargets[target.Target] = target
	}
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()

		targetName := params.Get("target")
		t := blackboxTargets[targetName]
		moduleName := params.Get("module")
		if moduleName == "" {
			params.Set("module", t.Module)
		}

		prober.Handler(w, r, bbh.modules, bbh.log, &prober.ResultHistory{}, bbh.cfg.ProbeTimeoutOffset, params, nil)
	}
}

// Static typecheck tests
var (
	_ integrations.Integration        = (*blackboxHandler)(nil)
	_ integrations.HTTPIntegration    = (*blackboxHandler)(nil)
	_ integrations.MetricsIntegration = (*blackboxHandler)(nil)
)

func (bbh *blackboxHandler) RunIntegration(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
