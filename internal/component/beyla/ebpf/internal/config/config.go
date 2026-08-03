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
func Build(args Arguments, rt Runtime, _ *slog.Logger) map[string]any {
	return args.Convert(rt)
}

// set assigns m to cfg[key] only when m has at least one entry.
func set(cfg map[string]any, key string, m map[string]any) {
	if len(m) > 0 {
		cfg[key] = m
	}
}

// Convert assembles the full Beyla YAML config map from Alloy Arguments.
func (a Arguments) Convert(rt Runtime) map[string]any {
	cfg := make(map[string]any)

	// Generated section translations.
	set(cfg, "discovery", a.Discovery.Convert())
	set(cfg, "ebpf", a.EBPF.Convert())
	set(cfg, "injector", a.Injector.Convert())
	set(cfg, "javaagent", a.Javaagent.Convert())
	set(cfg, "jvm_runtime_metrics", a.JVMRuntimeMetrics.Convert())
	set(cfg, "nodejs", a.Nodejs.Convert())
	set(cfg, "routes", a.Routes.Convert())
	set(cfg, "stats", a.Stats.Convert())

	// Hand-written sections: Alloy-managed/injected, multi-section, or special.
	// prometheus_export always carries the proxy port, so it is emitted unconditionally.
	cfg["prometheus_export"] = a.Metrics.Convert(rt)

	if addr := rt.HealthAddr; addr != "" {
		cfg["health_check"] = map[string]any{
			"unix_socket_path": addr,
		}
	}

	set(cfg, "attributes", a.Attributes.Convert())

	// Network flows are gated on metrics.features containing "network" OR the
	// deprecated network.enable flag. enable:true must always be present — Beyla
	// requires it to activate network flows.
	if a.Metrics.hasNetworkFeature() || a.Metrics.Network.Enable {
		networkFlows := a.Metrics.Network.Convert()
		networkFlows["enable"] = true
		cfg["network"] = networkFlows
	}

	set(cfg, "filter", a.Filters.Convert())

	if export := a.otelTracesExport(rt); len(export) > 0 {
		cfg["otel_traces_export"] = export
	}
	if export := a.otelMetricsExport(rt); len(export) > 0 {
		cfg["otel_metrics_export"] = export
	}

	// internal_metrics always carries the proxy port, so it is emitted unconditionally.
	cfg["internal_metrics"] = a.InternalMetrics.Convert(rt)

	if a.Debug && a.LogLevel == "" {
		cfg["log_level"] = "debug"
	} else if a.LogLevel != "" {
		// TODO: auto-derive from component.Options once Alloy exposes the level there
		cfg["log_level"] = a.LogLevel
	}

	if a.TracePrinter != "" && a.TracePrinter != "disabled" {
		cfg["trace_printer"] = a.TracePrinter
	}

	if a.EnforceSysCaps {
		cfg["enforce_sys_caps"] = true
	}

	return cfg
}

// Convert builds the prometheus_export configuration. rt.Port is the proxy port
// Beyla exposes its Prometheus endpoint on.
func (m Metrics) Convert(rt Runtime) map[string]any {
	prometheus := map[string]any{
		"port": rt.Port,
	}

	if v := m.Features; len(v) > 0 {
		prometheus["features"] = v
	}
	if v := m.Instrumentations; len(v) > 0 {
		prometheus["instrumentations"] = v
	}
	if m.AllowServiceGraphSelfReferences {
		prometheus["allow_service_graph_self_references"] = true
	}
	if v := m.ExtraResourceLabels; len(v) > 0 {
		prometheus["extra_resource_attributes"] = v
	}
	if v := m.ExtraSpanResourceLabels; len(v) > 0 {
		prometheus["extra_span_resource_attributes"] = v
	}
	if v := m.ExemplarFilter; v != "" {
		prometheus["exemplar_filter"] = v
	}
	if v := m.TTL; v != 0 {
		prometheus["ttl"] = v.String()
	}

	{
		bk := m.Buckets
		buckets := make(map[string]any)
		if len(bk.DurationHistogram) > 0 {
			buckets["duration_histogram"] = bk.DurationHistogram
		}
		if len(bk.RequestSizeHistogram) > 0 {
			buckets["request_size_histogram"] = bk.RequestSizeHistogram
		}
		if len(bk.ResponseSizeHistogram) > 0 {
			buckets["response_size_histogram"] = bk.ResponseSizeHistogram
		}
		if len(bk.StatTCPRTTHistogram) > 0 {
			buckets["stat_tcp_rtt_histogram"] = bk.StatTCPRTTHistogram
		}
		if len(bk.GenAIClientOperationDurationHistogram) > 0 {
			buckets["gen_ai_client_operation_duration_histogram"] = bk.GenAIClientOperationDurationHistogram
		}
		if len(bk.GenAIClientTokenUsageHistogram) > 0 {
			buckets["gen_ai_client_token_usage_histogram"] = bk.GenAIClientTokenUsageHistogram
		}
		set(prometheus, "buckets", buckets)
	}

	{
		nh := m.NativeHistogram
		nativeHistogram := make(map[string]any)
		if nh.BucketFactor != 0 {
			nativeHistogram["bucket_factor"] = nh.BucketFactor
		}
		if nh.MaxBucketNumber != 0 {
			nativeHistogram["max_bucket_number"] = nh.MaxBucketNumber
		}
		if nh.MinResetDuration != 0 {
			nativeHistogram["min_reset_duration"] = nh.MinResetDuration.String()
		}
		set(prometheus, "native_histogram", nativeHistogram)
	}

	return prometheus
}

// Convert builds the internal_metrics configuration. rt.Port is the proxy port
// Beyla's own beyla_internal_* metrics are exposed on.
func (im InternalMetrics) Convert(rt Runtime) map[string]any {
	m := make(map[string]any)

	if v := im.BpfMetricScrapeInterval; v != 0 {
		m["bpf_metric_scrape_interval"] = v.String()
	}

	// Default to the Prometheus exporter on the proxy port so Beyla's own
	// beyla_internal_* metrics (e.g. build_info) reach the scraped /metrics
	// endpoint. In-process Beyla published these via the shared registry; as a
	// subprocess it must be told explicitly. Override with internal_metrics.exporter.
	exporter := im.Exporter
	if exporter == "" {
		exporter = "prometheus"
	}
	m["exporter"] = exporter

	if exporter == "prometheus" {
		m["prometheus"] = map[string]any{
			"port": rt.Port,
			"path": "/metrics",
		}
	}

	return m
}

// Convert builds the attributes configuration.
func (a Attributes) Convert() map[string]any {
	attributes := make(map[string]any)

	// Kubernetes attributes
	if a.Kubernetes.Enable != "" {
		kubernetes := map[string]any{
			"enable": a.Kubernetes.Enable,
		}
		if a.Kubernetes.ClusterName != "" {
			kubernetes["cluster_name"] = a.Kubernetes.ClusterName
		}
		if a.Kubernetes.InformersSyncTimeout != 0 {
			kubernetes["informers_sync_timeout"] = a.Kubernetes.InformersSyncTimeout.String()
		}
		if a.Kubernetes.InformersResyncPeriod != 0 {
			kubernetes["informers_resync_period"] = a.Kubernetes.InformersResyncPeriod.String()
		}
		if len(a.Kubernetes.DisableInformers) > 0 {
			kubernetes["disable_informers"] = a.Kubernetes.DisableInformers
		}
		if a.Kubernetes.MetaRestrictLocalNode {
			kubernetes["meta_restrict_local_node"] = true
		}
		if a.Kubernetes.MetaCacheAddress != "" {
			kubernetes["meta_cache_address"] = a.Kubernetes.MetaCacheAddress
		}
		if a.Kubernetes.DropExternal {
			kubernetes["drop_external"] = true
		}
		if v := a.Kubernetes.KubeconfigPath; v != "" {
			kubernetes["kubeconfig_path"] = v
		}
		if v := a.Kubernetes.ResourceLabels; len(v) > 0 {
			kubernetes["resource_labels"] = v
		}
		if v := a.Kubernetes.ServiceNameTemplate; v != "" {
			kubernetes["service_name_template"] = v
		}
		attributes["kubernetes"] = kubernetes
	}

	// InstanceID attributes
	if a.InstanceID.HostnameDNSResolution != nil || a.InstanceID.OverrideHostname != "" {
		instanceID := make(map[string]any)
		if v := a.InstanceID.HostnameDNSResolution; v != nil {
			instanceID["dns"] = *v
		}
		if a.InstanceID.OverrideHostname != "" {
			instanceID["override_hostname"] = a.InstanceID.OverrideHostname
		}
		attributes["instance_id"] = instanceID
	}

	// Select attributes
	if len(a.Select) > 0 {
		set(attributes, "select", a.Select.Convert())
	}

	if v := a.HostID.Override; v != "" {
		attributes["host_id"] = map[string]any{"override": v}
	}

	{
		r := a.MetadataRetry
		metadataRetry := make(map[string]any)
		if r.Timeout != 0 {
			metadataRetry["timeout"] = r.Timeout.String()
		}
		if r.StartInterval != 0 {
			metadataRetry["start_interval"] = r.StartInterval.String()
		}
		if r.MaxInterval != 0 {
			metadataRetry["max_interval"] = r.MaxInterval.String()
		}
		set(attributes, "metadata_retry", metadataRetry)
	}

	if v := a.MetricSpanNamesLimit; v != 0 {
		attributes["metric_span_names_limit"] = v
	}
	if v := a.RenameUnresolvedHosts; v != "" {
		attributes["rename_unresolved_hosts"] = v
	}
	if v := a.RenameUnresolvedHostsIncoming; v != "" {
		attributes["rename_unresolved_hosts_incoming"] = v
	}
	if v := a.RenameUnresolvedHostsOutgoing; v != "" {
		attributes["rename_unresolved_hosts_outgoing"] = v
	}

	return attributes
}

// Convert builds the filter configuration.
func (f Filters) Convert() map[string]any {
	if len(f.Application) == 0 && len(f.Network) == 0 {
		return nil
	}

	filters := make(map[string]any)

	if len(f.Application) > 0 {
		set(filters, "application", f.Application.Convert())
	}

	if len(f.Network) > 0 {
		set(filters, "network", f.Network.Convert())
	}

	return filters
}

// otelTracesExport builds the otel_traces_export section. It needs a.Output
// (to know traces are wired) and rt.OTLPAddr (the receiver socket).
func (a Arguments) otelTracesExport(rt Runtime) map[string]any {
	if a.Output == nil || len(a.Output.Traces) == 0 {
		return nil
	}

	addr := rt.OTLPAddr
	if addr == "" {
		return nil
	}

	endpoint := "unix://" + addr
	export := map[string]any{
		"endpoint": endpoint,
		"protocol": "http/protobuf",
	}

	if len(a.Traces.Instrumentations) > 0 {
		export["instrumentations"] = a.Traces.Instrumentations
	}

	if a.Traces.Sampler.Name != "" {
		sampler := map[string]any{
			"name": a.Traces.Sampler.Name,
		}
		if a.Traces.Sampler.Arg != "" {
			sampler["arg"] = a.Traces.Sampler.Arg
		}
		export["sampler"] = sampler
	}

	return export
}

// otelMetricsExport builds the otel_metrics_export section. It needs a.Output
// (to know metrics are wired) and rt.OTLPAddr (the receiver socket).
func (a Arguments) otelMetricsExport(rt Runtime) map[string]any {
	if a.Output == nil || len(a.Output.Metrics) == 0 {
		return nil
	}

	addr := rt.OTLPAddr
	if addr == "" {
		return nil
	}

	endpoint := "unix://" + addr
	return map[string]any{
		"endpoint": endpoint,
		"protocol": "http/protobuf",
	}
}
