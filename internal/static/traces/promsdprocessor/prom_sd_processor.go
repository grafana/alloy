package promsdprocessor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging"
	promsdconsumer "github.com/grafana/alloy/internal/static/traces/promsdprocessor/consumer"
	util "github.com/grafana/alloy/internal/util/log"
)

type promServiceDiscoProcessor struct {
	discoveryMgr     *promdiscovery.Manager
	discoveryMgrStop context.CancelFunc
	discoveryMgrCtx  context.Context

	relabelConfigs map[string][]*relabel.Config

	consumer *promsdconsumer.Consumer

	logger log.Logger
}

func newTraceProcessor(nextConsumer consumer.Traces, operationType string, podAssociations []string, scrapeConfigs []*config.ScrapeConfig) (processor.Traces, error) {
	ctx, cancel := context.WithCancel(context.Background())

	logger := log.With(util.Logger, "component", "traces service disco")

	// NOTE: Since this static mode code path is only used for converter code, we can safely use the default registerer.
	discoveryManagerRegistry := prometheus.DefaultRegisterer
	sdMetrics, err := promdiscovery.CreateAndRegisterSDMetrics(prometheus.DefaultRegisterer)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create prometheus service discovery metrics %w", err)
	}

	mgr := promdiscovery.NewManager(
		ctx,
		slog.New(logging.NewSlogGoKitHandler(logger)),
		discoveryManagerRegistry,
		sdMetrics,
		promdiscovery.Name("traces service disco"),
	)

	relabelConfigs := map[string][]*relabel.Config{}
	managerConfig := map[string]promdiscovery.Configs{}
	for _, v := range scrapeConfigs {
		for _, rule := range v.RelabelConfigs {
			if rule == nil {
				continue
			}
			// The relabel package expects a concrete validation scheme. Explicitly
			// set legacy validation until we expose this as a user-facing option.
			if rule.NameValidationScheme == model.UnsetValidation {
				rule.NameValidationScheme = model.LegacyValidation
			}
		}
		managerConfig[v.JobName] = v.ServiceDiscoveryConfigs
		relabelConfigs[v.JobName] = v.RelabelConfigs
	}

	err = mgr.ApplyConfig(managerConfig)
	if err != nil {
		cancel()
		return nil, err
	}

	if len(podAssociations) == 0 {
		podAssociations = []string{
			promsdconsumer.PodAssociationIPLabel,
			promsdconsumer.PodAssociationOTelIPLabel,
			promsdconsumer.PodAssociationk8sIPLabel,
			promsdconsumer.PodAssociationHostnameLabel,
			promsdconsumer.PodAssociationConnectionIP,
		}
	}

	consumerOpts := promsdconsumer.Options{
		// Don't bother setting up labels - this will be done by the UpdateOptionsHostLabels() function.
		HostLabels:      map[string]discovery.Target{},
		OperationType:   operationType,
		PodAssociations: podAssociations,
		NextConsumer:    nextConsumer,
	}
	consumer, err := promsdconsumer.NewConsumer(consumerOpts, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create a new consumer %w", err)
	}

	return &promServiceDiscoProcessor{
		discoveryMgr:     mgr,
		discoveryMgrStop: cancel,
		discoveryMgrCtx:  ctx,
		relabelConfigs:   relabelConfigs,
		logger:           logger,
		consumer:         consumer,
	}, nil
}

func (p *promServiceDiscoProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return p.consumer.ConsumeTraces(ctx, td)
}

func (p *promServiceDiscoProcessor) Capabilities() consumer.Capabilities {
	return p.consumer.Capabilities()
}

// Start is invoked during service startup.
func (p *promServiceDiscoProcessor) Start(_ context.Context, _ component.Host) error {
	go p.watchServiceDiscovery()

	go func() {
		err := p.discoveryMgr.Run()
		if err != nil && err != context.Canceled {
			level.Error(p.logger).Log("msg", "failed to start prom svc disco.  relabeling disabled", "err", err)
		}
	}()

	return nil
}

// Shutdown is invoked during service shutdown.
func (p *promServiceDiscoProcessor) Shutdown(context.Context) error {
	if p.discoveryMgrStop != nil {
		p.discoveryMgrStop()
	}
	return nil
}

func (p *promServiceDiscoProcessor) watchServiceDiscovery() {
	for {
		// p.discoveryMgr.SyncCh() is never closed so we need to watch the context as well to properly exit this goroutine
		select {
		case targetGroups := <-p.discoveryMgr.SyncCh():
			hostLabels := make(map[string]discovery.Target)
			level.Debug(p.logger).Log("msg", "syncing target groups", "count", len(targetGroups))
			for jobName, groups := range targetGroups {
				p.syncGroups(jobName, groups, hostLabels)
			}
			p.consumer.UpdateOptionsHostLabels(hostLabels)
		case <-p.discoveryMgrCtx.Done():
			return
		}
	}
}

func (p *promServiceDiscoProcessor) syncGroups(jobName string, groups []*targetgroup.Group, hostLabels map[string]discovery.Target) {
	level.Debug(p.logger).Log("msg", "syncing target group", "jobName", jobName)
	for _, g := range groups {
		p.syncTargets(jobName, g, hostLabels)
	}
}

func (p *promServiceDiscoProcessor) syncTargets(jobName string, group *targetgroup.Group, hostLabels map[string]discovery.Target) {
	level.Debug(p.logger).Log("msg", "syncing targets", "count", len(group.Targets))

	relabelConfig := p.relabelConfigs[jobName]
	if relabelConfig == nil {
		level.Warn(p.logger).Log("msg", "relabel config not found for job. skipping labeling", "jobName", jobName)
		return
	}

	for _, t := range group.Targets {
		discoveredLabels := group.Labels.Merge(t)

		level.Debug(p.logger).Log("discoveredLabels", discoveredLabels)
		var labelMap = make(map[string]string)
		for k, v := range discoveredLabels.Clone() {
			labelMap[string(k)] = string(v)
		}
		processedLabels, keep := relabel.Process(labels.FromMap(labelMap), relabelConfig...)
		level.Debug(p.logger).Log("processedLabels", processedLabels)
		if !keep {
			continue
		}

		var labels = discovery.NewTargetFromModelLabels(processedLabels)
		host, err := promsdconsumer.GetHostFromLabels(labels)
		if err != nil {
			level.Warn(p.logger).Log("msg", "ignoring target, unable to find address", "err", err)
			continue
		}

		level.Debug(p.logger).Log("msg", "adding host to hostLabels", "host", host)
		hostLabels[host] = promsdconsumer.NewTargetsWithNonInternalLabels(labels)
	}
}
