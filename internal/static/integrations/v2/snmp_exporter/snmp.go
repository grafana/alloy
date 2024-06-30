package snmp_exporter_v2

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/static/integrations/snmp_exporter"
	"github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/autoscrape"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/go-kit/log"
	snmp_config "github.com/prometheus/snmp_exporter/config"
)

type snmpHandler struct {
	cfg     *Config
	snmpCfg *snmp_config.Config
	log     log.Logger
}

func (sh *snmpHandler) Targets(ep integrations.Endpoint) []*targetgroup.Group {
	integrationNameValue := model.LabelValue("integrations/" + sh.cfg.Name())
	key, _ := sh.cfg.Identifier(sh.cfg.globals)

	group := &targetgroup.Group{
		Labels: model.LabelSet{
			model.InstanceLabel: model.LabelValue(key),
			model.JobLabel:      integrationNameValue,
			"agent_hostname":    model.LabelValue(sh.cfg.globals.AgentIdentifier),

			// Meta labels that can be used during SD.
			"__meta_agent_integration_name":       model.LabelValue(sh.cfg.Name()),
			"__meta_agent_integration_instance":   model.LabelValue(sh.cfg.Name()),
			"__meta_agent_integration_autoscrape": model.LabelValue(metricsutils.BoolToString(*sh.cfg.Common.Autoscrape.Enable)),
		},
		Source: fmt.Sprintf("%s/%s", sh.cfg.Name(), sh.cfg.Name()),
	}

	for _, lbl := range sh.cfg.Common.ExtraLabels {
		group.Labels[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	}

	for _, t := range sh.cfg.SnmpTargets {
		labelSet := model.LabelSet{
			model.AddressLabel:     model.LabelValue(ep.Host),
			model.MetricsPathLabel: model.LabelValue(path.Join(ep.Prefix, "metrics")),
			"snmp_target":          model.LabelValue(t.Target),
			"__param_target":       model.LabelValue(t.Target),
		}

		if t.Module != "" {
			labelSet = labelSet.Merge(model.LabelSet{
				"__param_module": model.LabelValue(t.Module),
			})
		}

		if t.WalkParams != "" {
			labelSet = labelSet.Merge(model.LabelSet{
				"__param_walk_params": model.LabelValue(t.WalkParams),
			})
		}

		if t.SNMPContext != "" {
			labelSet = labelSet.Merge(model.LabelSet{
				"__param_snmp_context": model.LabelValue(t.SNMPContext),
			})
		}

		if t.Auth != "" {
			labelSet = labelSet.Merge(model.LabelSet{
				"__param_auth": model.LabelValue(t.Auth),
			})
		}
		group.Targets = append(group.Targets, labelSet)
	}

	return []*targetgroup.Group{group}
}

func (sh *snmpHandler) ScrapeConfigs(sd discovery.Configs) []*autoscrape.ScrapeConfig {
	if !*sh.cfg.Common.Autoscrape.Enable {
		return nil
	}
	name := sh.cfg.Name()
	cfg := config.DefaultScrapeConfig
	cfg.JobName = fmt.Sprintf("%s/%s", name, name)
	cfg.Scheme = sh.cfg.globals.AgentBaseURL.Scheme
	cfg.ServiceDiscoveryConfigs = sd
	cfg.ScrapeInterval = sh.cfg.Common.Autoscrape.ScrapeInterval
	cfg.ScrapeTimeout = sh.cfg.Common.Autoscrape.ScrapeTimeout
	cfg.RelabelConfigs = sh.cfg.Common.Autoscrape.RelabelConfigs
	cfg.MetricRelabelConfigs = sh.cfg.Common.Autoscrape.MetricRelabelConfigs

	return []*autoscrape.ScrapeConfig{{
		Instance: sh.cfg.Common.Autoscrape.MetricsInstance,
		Config:   cfg,
	}}
}

func (sh *snmpHandler) Handler(prefix string) (http.Handler, error) {
	r := mux.NewRouter()
	r.Handle(path.Join(prefix, "metrics"), sh.createHandler())

	return r, nil
}

// Static typecheck tests
var (
	_ integrations.Integration        = (*snmpHandler)(nil)
	_ integrations.HTTPIntegration    = (*snmpHandler)(nil)
	_ integrations.MetricsIntegration = (*snmpHandler)(nil)
)

func (sh *snmpHandler) RunIntegration(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (sh *snmpHandler) createHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snmp_exporter.Handler(w, r, sh.log, sh.snmpCfg, sh.cfg.SnmpTargets, sh.cfg.WalkParams)
	}
}
