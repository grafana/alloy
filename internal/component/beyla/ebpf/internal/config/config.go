//go:generate go run ./gen/main.go ./gen/skeleton.go

package config

import "log/slog"

// Runtime carries the values Alloy injects at launch (not user config): the
// proxy/metrics port, the health-check socket path, and the OTLP receiver addr.
type Runtime struct {
	Port       int
	HealthAddr string
	OTLPAddr   string
}

// Build translates Alloy Arguments into the Beyla YAML config map.
func Build(args Arguments, rt Runtime, log *slog.Logger) map[string]any {
	b := &builder{args: args, rt: rt, log: log}
	return b.build()
}

type builder struct {
	args Arguments
	rt   Runtime
	log  *slog.Logger
}

func (b *builder) build() map[string]any {
	config := make(map[string]any)

	// Generated section translations (discovery, ebpf, routes, stats, injector,
	// javaagent, nodejs, …); addGeneratedConfig calls each.
	b.addGeneratedConfig(config)

	// Hand-written sections: Alloy-managed/injected, multi-section, or special.
	b.addPrometheusConfig(config)
	b.addHealthCheckConfig(config)
	b.addAttributesConfig(config)
	b.addNetworkFlowsConfig(config)
	b.addFiltersConfig(config)
	b.addTracesConfig(config)
	b.addOTLPTracesExportConfig(config)
	b.addOTLPMetricsExportConfig(config)
	b.addInternalMetricsConfig(config)
	b.addLogLevelConfig(config)
	b.addTracePrinterConfig(config)
	b.addEnforceSysCapsConfig(config)

	return config
}

func (b *builder) addPrometheusConfig(config map[string]any) {
	prometheus := map[string]any{
		"port": b.rt.Port,
	}

	b.fillPrometheusExportConfig(prometheus)

	if m := b.buildBucketsConfig(); len(m) > 0 {
		prometheus["buckets"] = m
	}
	if m := b.buildNativeHistogramConfig(); len(m) > 0 {
		prometheus["native_histogram"] = m
	}

	config["prometheus_export"] = prometheus
}

func (b *builder) buildBucketsConfig() map[string]any {
	bk := b.args.Metrics.Buckets
	m := make(map[string]any)

	if len(bk.DurationHistogram) > 0 {
		m["duration_histogram"] = bk.DurationHistogram
	}
	if len(bk.RequestSizeHistogram) > 0 {
		m["request_size_histogram"] = bk.RequestSizeHistogram
	}
	if len(bk.ResponseSizeHistogram) > 0 {
		m["response_size_histogram"] = bk.ResponseSizeHistogram
	}
	if len(bk.StatTCPRTTHistogram) > 0 {
		m["stat_tcp_rtt_histogram"] = bk.StatTCPRTTHistogram
	}
	if len(bk.GenAIClientOperationDurationHistogram) > 0 {
		m["gen_ai_client_operation_duration_histogram"] = bk.GenAIClientOperationDurationHistogram
	}
	if len(bk.GenAIClientTokenUsageHistogram) > 0 {
		m["gen_ai_client_token_usage_histogram"] = bk.GenAIClientTokenUsageHistogram
	}

	return m
}

func (b *builder) buildNativeHistogramConfig() map[string]any {
	nh := b.args.Metrics.NativeHistogram
	m := make(map[string]any)

	if nh.BucketFactor != 0 {
		m["bucket_factor"] = nh.BucketFactor
	}
	if nh.MaxBucketNumber != 0 {
		m["max_bucket_number"] = nh.MaxBucketNumber
	}
	if nh.MinResetDuration != 0 {
		m["min_reset_duration"] = nh.MinResetDuration.String()
	}

	return m
}

func (b *builder) addHealthCheckConfig(config map[string]any) {
	addr := b.rt.HealthAddr

	if addr == "" {
		return
	}

	config["health_check"] = map[string]any{
		"unix_socket_path": addr,
	}
}

func (b *builder) addInternalMetricsConfig(config map[string]any) {
	m := make(map[string]any)

	if v := b.args.InternalMetrics.BpfMetricScrapeInterval; v != 0 {
		m["bpf_metric_scrape_interval"] = v.String()
	}

	// Default to the Prometheus exporter on the proxy port so Beyla's own
	// beyla_internal_* metrics (e.g. build_info) reach the scraped /metrics
	// endpoint. In-process Beyla published these via the shared registry; as a
	// subprocess it must be told explicitly. Override with internal_metrics.exporter.
	exporter := b.args.InternalMetrics.Exporter
	if exporter == "" {
		exporter = "prometheus"
	}
	m["exporter"] = exporter

	if exporter == "prometheus" {
		m["prometheus"] = map[string]any{
			"port": b.rt.Port,
			"path": "/metrics",
		}
	}

	config["internal_metrics"] = m
}

func (b *builder) addAttributesConfig(config map[string]any) {
	attributes := make(map[string]any)

	// Kubernetes attributes
	if b.args.Attributes.Kubernetes.Enable != "" {
		attributes["kubernetes"] = b.buildKubernetesConfig()
	}

	// InstanceID attributes
	if b.args.Attributes.InstanceID.HostnameDNSResolution != nil || b.args.Attributes.InstanceID.OverrideHostname != "" {
		attributes["instance_id"] = b.buildInstanceIDConfig()
	}

	// Select attributes
	if len(b.args.Attributes.Select) > 0 {
		if selectMap := b.buildSelectConfig(); len(selectMap) > 0 {
			attributes["select"] = selectMap
		}
	}

	if v := b.args.Attributes.HostID.Override; v != "" {
		attributes["host_id"] = map[string]any{"override": v}
	}
	if m := b.buildMetadataRetryConfig(); len(m) > 0 {
		attributes["metadata_retry"] = m
	}
	if v := b.args.Attributes.MetricSpanNamesLimit; v != 0 {
		attributes["metric_span_names_limit"] = v
	}
	if v := b.args.Attributes.RenameUnresolvedHosts; v != "" {
		attributes["rename_unresolved_hosts"] = v
	}
	if v := b.args.Attributes.RenameUnresolvedHostsIncoming; v != "" {
		attributes["rename_unresolved_hosts_incoming"] = v
	}
	if v := b.args.Attributes.RenameUnresolvedHostsOutgoing; v != "" {
		attributes["rename_unresolved_hosts_outgoing"] = v
	}

	if len(attributes) > 0 {
		config["attributes"] = attributes
	}
}

func (b *builder) buildMetadataRetryConfig() map[string]any {
	r := b.args.Attributes.MetadataRetry
	m := make(map[string]any)

	if r.Timeout != 0 {
		m["timeout"] = r.Timeout.String()
	}
	if r.StartInterval != 0 {
		m["start_interval"] = r.StartInterval.String()
	}
	if r.MaxInterval != 0 {
		m["max_interval"] = r.MaxInterval.String()
	}

	return m
}

func (b *builder) buildKubernetesConfig() map[string]any {
	kubernetes := map[string]any{
		"enable": b.args.Attributes.Kubernetes.Enable,
	}

	if b.args.Attributes.Kubernetes.ClusterName != "" {
		kubernetes["cluster_name"] = b.args.Attributes.Kubernetes.ClusterName
	}
	if b.args.Attributes.Kubernetes.InformersSyncTimeout != 0 {
		kubernetes["informers_sync_timeout"] = b.args.Attributes.Kubernetes.InformersSyncTimeout.String()
	}
	if b.args.Attributes.Kubernetes.InformersResyncPeriod != 0 {
		kubernetes["informers_resync_period"] = b.args.Attributes.Kubernetes.InformersResyncPeriod.String()
	}
	if len(b.args.Attributes.Kubernetes.DisableInformers) > 0 {
		kubernetes["disable_informers"] = b.args.Attributes.Kubernetes.DisableInformers
	}
	if b.args.Attributes.Kubernetes.MetaRestrictLocalNode {
		kubernetes["meta_restrict_local_node"] = true
	}
	if b.args.Attributes.Kubernetes.MetaCacheAddress != "" {
		kubernetes["meta_cache_address"] = b.args.Attributes.Kubernetes.MetaCacheAddress
	}
	if b.args.Attributes.Kubernetes.DropExternal {
		kubernetes["drop_external"] = true
	}
	if v := b.args.Attributes.Kubernetes.KubeconfigPath; v != "" {
		kubernetes["kubeconfig_path"] = v
	}
	if v := b.args.Attributes.Kubernetes.ResourceLabels; len(v) > 0 {
		kubernetes["resource_labels"] = v
	}
	if v := b.args.Attributes.Kubernetes.ServiceNameTemplate; v != "" {
		kubernetes["service_name_template"] = v
	}

	return kubernetes
}

func (b *builder) buildInstanceIDConfig() map[string]any {
	instanceID := make(map[string]any)

	if v := b.args.Attributes.InstanceID.HostnameDNSResolution; v != nil {
		instanceID["dns"] = *v
	}
	if b.args.Attributes.InstanceID.OverrideHostname != "" {
		instanceID["override_hostname"] = b.args.Attributes.InstanceID.OverrideHostname
	}

	return instanceID
}

func (b *builder) buildSelectConfig() map[string]any {
	selectMap := make(map[string]any)

	for _, sel := range b.args.Attributes.Select {
		selConfig := make(map[string]any)
		if len(sel.Include) > 0 {
			selConfig["include"] = sel.Include
		}
		if len(sel.Exclude) > 0 {
			selConfig["exclude"] = sel.Exclude
		}
		if len(selConfig) > 0 {
			selectMap[sel.Section] = selConfig
		}
	}

	return selectMap
}

func (b *builder) addNetworkFlowsConfig(config map[string]any) {
	// Gated on metrics.features containing "network" OR the deprecated network.enable flag.
	// enable:true must always be present — Beyla requires it to activate network flows.
	if !b.args.Metrics.hasNetworkFeature() && !b.args.Metrics.Network.Enable {
		return
	}

	networkFlows := map[string]any{
		"enable": true,
	}

	b.fillNetworkConfig(networkFlows)

	config["network"] = networkFlows
}

func (b *builder) addFiltersConfig(config map[string]any) {
	if len(b.args.Filters.Application) == 0 && len(b.args.Filters.Network) == 0 {
		return
	}

	filters := make(map[string]any)

	if len(b.args.Filters.Application) > 0 {
		app := make(map[string]any)
		fillAttributeFamiliesConfig(app, b.args.Filters.Application)
		if len(app) > 0 {
			filters["application"] = app
		}
	}

	if len(b.args.Filters.Network) > 0 {
		net := make(map[string]any)
		fillAttributeFamiliesConfig(net, b.args.Filters.Network)
		if len(net) > 0 {
			filters["network"] = net
		}
	}

	if len(filters) > 0 {
		config["filters"] = filters
	}
}

func (b *builder) addTracesConfig(config map[string]any) {
	if len(b.args.Traces.Instrumentations) == 0 && b.args.Traces.Sampler.Name == "" {
		return
	}

	traces := make(map[string]any)

	if len(b.args.Traces.Instrumentations) > 0 {
		traces["instrumentations"] = b.args.Traces.Instrumentations
	}

	if b.args.Traces.Sampler.Name != "" {
		sampler := map[string]any{
			"name": b.args.Traces.Sampler.Name,
		}
		if b.args.Traces.Sampler.Arg != "" {
			sampler["arg"] = b.args.Traces.Sampler.Arg
		}
		traces["sampler"] = sampler
	}

	if len(traces) > 0 {
		config["traces"] = traces
	}
}

func (b *builder) addOTLPTracesExportConfig(config map[string]any) {
	if b.args.Output == nil || len(b.args.Output.Traces) == 0 {
		return
	}

	addr := b.rt.OTLPAddr
	if addr == "" {
		return
	}

	endpoint := "unix://" + addr
	config["otel_traces_export"] = map[string]any{
		"endpoint": endpoint,
		"protocol": "http/protobuf",
	}
	b.log.Debug("configured OTLP traces export", "endpoint", endpoint)
}

func (b *builder) addOTLPMetricsExportConfig(config map[string]any) {
	if b.args.Output == nil || len(b.args.Output.Metrics) == 0 {
		return
	}

	addr := b.rt.OTLPAddr
	if addr == "" {
		return
	}

	endpoint := "unix://" + addr
	config["otel_metrics_export"] = map[string]any{
		"endpoint": endpoint,
		"protocol": "http/protobuf",
	}
	b.log.Debug("configured OTLP metrics export", "endpoint", endpoint)
}

func (b *builder) addLogLevelConfig(config map[string]any) {
	if b.args.Debug && b.args.LogLevel == "" {
		config["log_level"] = "debug"
		return
	}

	if b.args.LogLevel != "" {
		// TODO: auto-derive from component.Options once Alloy exposes the level there
		config["log_level"] = b.args.LogLevel
	}
}

func (b *builder) addTracePrinterConfig(config map[string]any) {
	if b.args.TracePrinter != "" && b.args.TracePrinter != "disabled" {
		config["trace_printer"] = b.args.TracePrinter
	}
}

func (b *builder) addEnforceSysCapsConfig(config map[string]any) {
	if b.args.EnforceSysCaps {
		config["enforce_sys_caps"] = true
	}
}
