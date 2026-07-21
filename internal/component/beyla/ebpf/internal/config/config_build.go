package config

// Convert builds the discovery configuration.
func (d Discovery) Convert() map[string]any {
	m := make(map[string]any)
	if d.BpfPidFilterOff {
		m["bpf_pid_filter_off"] = true
	}
	if len(d.DefaultExcludeInstrument) > 0 {
		m["default_exclude_instrument"] = d.DefaultExcludeInstrument.Convert()
	}
	if len(d.DefaultExcludeServices) > 0 {
		m["default_exclude_services"] = d.DefaultExcludeServices.Convert()
	}
	if v := d.DefaultOtlpGrpcPort; v != 0 {
		m["default_otlp_grpc_port"] = v
	}
	if v := d.DisabledRouteHarvesters; len(v) > 0 {
		m["disabled_route_harvesters"] = v
	}
	if len(d.ExcludeInstrument) > 0 {
		m["exclude_instrument"] = d.ExcludeInstrument.Convert()
	}
	if v := d.ExcludeOTelInstrumentedServices; v != nil {
		m["exclude_otel_instrumented_services"] = *v
	}
	if d.ExcludeOtelInstrumentedServicesSpanMetrics {
		m["exclude_otel_instrumented_services_span_metrics"] = true
	}
	if len(d.ExcludeServices) > 0 {
		m["exclude_services"] = d.ExcludeServices.Convert()
	}
	if v := d.ExcludedLinuxSystemPaths; len(v) > 0 {
		m["excluded_linux_system_paths"] = v
	}
	if len(d.Instrument) > 0 {
		m["instrument"] = d.Instrument.Convert()
	}
	if v := d.MinProcessAge; v != 0 {
		m["min_process_age"] = v.String()
	}
	if v := d.PollInterval; v != 0 {
		m["poll_interval"] = v.String()
	}
	{
		m1 := make(map[string]any)
		if v := d.RouteHarvesterAdvanced.JavaHarvestDelay; v != 0 {
			m1["java_harvest_delay"] = v.String()
		}
		if len(m1) > 0 {
			m["route_harvester_advanced"] = m1
		}
	}
	if v := d.RouteHarvesterTimeout; v != 0 {
		m["route_harvester_timeout"] = v.String()
	}
	if len(d.Services) > 0 {
		m["services"] = d.Services.Convert()
	}
	if d.SkipGoSpecificTracers {
		m["skip_go_specific_tracers"] = true
	}
	if len(d.Survey) > 0 {
		m["survey"] = d.Survey.Convert()
	}
	return m
}

// Convert builds the ebpf configuration.
func (e EBPF) Convert() map[string]any {
	m := make(map[string]any)
	if v := e.BatchLength; v != 0 {
		m["batch_length"] = v
	}
	if v := e.BatchTimeout; v != 0 {
		m["batch_timeout"] = v.String()
	}
	if e.BpfDebug {
		m["bpf_debug"] = true
	}
	if v := e.BpfFsPath; v != "" {
		m["bpf_fs_path"] = v
	}
	{
		m1 := make(map[string]any)
		if v := e.BufferSizes.Http; v != 0 {
			m1["http"] = v
		}
		if v := e.BufferSizes.Kafka; v != 0 {
			m1["kafka"] = v
		}
		if v := e.BufferSizes.Mssql; v != 0 {
			m1["mssql"] = v
		}
		if v := e.BufferSizes.Mysql; v != 0 {
			m1["mysql"] = v
		}
		if v := e.BufferSizes.Postgres; v != 0 {
			m1["postgres"] = v
		}
		if v := e.BufferSizes.Tcp; v != 0 {
			m1["tcp"] = v
		}
		if len(m1) > 0 {
			m["buffer_sizes"] = m1
		}
	}
	if v := e.ContextPropagation; v != "" {
		m["context_propagation"] = v
	}
	if v := e.CouchbaseDbCacheSize; v != 0 {
		m["couchbase_db_cache_size"] = v
	}
	if e.DisableBlackBoxCp {
		m["disable_black_box_cp"] = true
	}
	if v := e.DnsRequestTimeout; v != 0 {
		m["dns_request_timeout"] = v.String()
	}
	if v := e.ForceBpfMapReader; v != "" {
		m["force_bpf_map_reader"] = v
	}
	if e.HeuristicSQLDetect {
		m["heuristic_sql_detect"] = true
	}
	if e.HighRequestVolume {
		m["high_request_volume"] = true
	}
	if v := e.HTTPRequestTimeout; v != 0 {
		m["http_request_timeout"] = v.String()
	}
	if v := e.InstrumentCuda; v != 0 {
		m["instrument_cuda"] = v
	}
	if v := e.KafkaTopicUuidCacheSize; v != 0 {
		m["kafka_topic_uuid_cache_size"] = v
	}
	{
		m1 := make(map[string]any)
		if v := e.LogEnricher.AsyncWriterChannelLen; v != 0 {
			m1["async_writer_channel_len"] = v
		}
		if v := e.LogEnricher.AsyncWriterWorkers; v != 0 {
			m1["async_writer_workers"] = v
		}
		if v := e.LogEnricher.CacheSize; v != 0 {
			m1["cache_size"] = v
		}
		if v := e.LogEnricher.CacheTtl; v != 0 {
			m1["cache_ttl"] = v.String()
		}
		{
			list2 := make([]map[string]any, 0, len(e.LogEnricher.Services))
			for _, item2 := range e.LogEnricher.Services {
				m2 := make(map[string]any)
				if len(item2.Service) > 0 {
					m2["service"] = item2.Service.Convert()
				}
				if len(m2) > 0 {
					list2 = append(list2, m2)
				}
			}
			if len(list2) > 0 {
				m1["services"] = list2
			}
		}
		if len(m1) > 0 {
			m["log_enricher"] = m1
		}
	}
	{
		m1 := make(map[string]any)
		if v := e.MapsConfig.GlobalScaleFactor; v != 0 {
			m1["global_scale_factor"] = v
		}
		if len(m1) > 0 {
			m["maps_config"] = m1
		}
	}
	if v := e.MaxTransactionTime; v != 0 {
		m["max_transaction_time"] = v.String()
	}
	if v := e.MongoRequestsCacheSize; v != 0 {
		m["mongo_requests_cache_size"] = v
	}
	if v := e.MssqlPreparedStatementsCacheSize; v != 0 {
		m["mssql_prepared_statements_cache_size"] = v
	}
	if v := e.MysqlPreparedStatementsCacheSize; v != 0 {
		m["mysql_prepared_statements_cache_size"] = v
	}
	if e.OverrideBpfloopEnabled {
		m["override_bpfloop_enabled"] = true
	}
	{
		m1 := make(map[string]any)
		{
			m2 := make(map[string]any)
			{
				m3 := make(map[string]any)
				if e.PayloadExtraction.HTTP.Aws.Enabled {
					m3["enabled"] = true
				}
				if len(m3) > 0 {
					m2["aws"] = m3
				}
			}
			{
				m3 := make(map[string]any)
				if e.PayloadExtraction.HTTP.Elasticsearch.Enabled {
					m3["enabled"] = true
				}
				if len(m3) > 0 {
					m2["elasticsearch"] = m3
				}
			}
			{
				m3 := make(map[string]any)
				if e.PayloadExtraction.HTTP.Enrichment.Enabled {
					m3["enabled"] = true
				}
				{
					m4 := make(map[string]any)
					{
						m5 := make(map[string]any)
						if v := e.PayloadExtraction.HTTP.Enrichment.Policy.DefaultAction.Body; v != "" {
							m5["body"] = v
						}
						if v := e.PayloadExtraction.HTTP.Enrichment.Policy.DefaultAction.Headers; v != "" {
							m5["headers"] = v
						}
						if len(m5) > 0 {
							m4["default_action"] = m5
						}
					}
					if v := e.PayloadExtraction.HTTP.Enrichment.Policy.ObfuscationString; v != "" {
						m4["obfuscation_string"] = v
					}
					if len(m4) > 0 {
						m3["policy"] = m4
					}
				}
				{
					list4 := make([]map[string]any, 0, len(e.PayloadExtraction.HTTP.Enrichment.Rules))
					for _, item4 := range e.PayloadExtraction.HTTP.Enrichment.Rules {
						m4 := make(map[string]any)
						if v := item4.Action; v != "" {
							m4["action"] = v
						}
						{
							m5 := make(map[string]any)
							if item4.Match.CaseSensitive {
								m5["case_sensitive"] = true
							}
							if v := item4.Match.Methods; len(v) > 0 {
								m5["methods"] = v
							}
							if v := item4.Match.ObfuscationJsonPaths; len(v) > 0 {
								m5["obfuscation_json_paths"] = v
							}
							if v := item4.Match.Patterns; len(v) > 0 {
								m5["patterns"] = v
							}
							if v := item4.Match.UrlPathPatterns; len(v) > 0 {
								m5["url_path_patterns"] = v
							}
							if len(m5) > 0 {
								m4["match"] = m5
							}
						}
						if v := item4.Scope; v != "" {
							m4["scope"] = v
						}
						if v := item4.Type; v != "" {
							m4["type"] = v
						}
						if len(m4) > 0 {
							list4 = append(list4, m4)
						}
					}
					if len(list4) > 0 {
						m3["rules"] = list4
					}
				}
				if len(m3) > 0 {
					m2["enrichment"] = m3
				}
			}
			{
				m3 := make(map[string]any)
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.OpenAI.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["openai"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Anthropic.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["anthropic"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Gemini.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["gemini"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Bedrock.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["bedrock"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Embedding.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["embedding"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Mcp.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["mcp"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Qwen.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["qwen"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Rerank.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["rerank"] = m4
					}
				}
				{
					m4 := make(map[string]any)
					if e.PayloadExtraction.HTTP.Retrieval.Enabled {
						m4["enabled"] = true
					}
					if len(m4) > 0 {
						m3["retrieval"] = m4
					}
				}
				if len(m3) > 0 {
					m2["genai"] = m3
				}
			}
			{
				m3 := make(map[string]any)
				if e.PayloadExtraction.HTTP.Graphql.Enabled {
					m3["enabled"] = true
				}
				if len(m3) > 0 {
					m2["graphql"] = m3
				}
			}
			{
				m3 := make(map[string]any)
				if e.PayloadExtraction.HTTP.Jsonrpc.Enabled {
					m3["enabled"] = true
				}
				if len(m3) > 0 {
					m2["jsonrpc"] = m3
				}
			}
			{
				m3 := make(map[string]any)
				if e.PayloadExtraction.HTTP.Sqlpp.Enabled {
					m3["enabled"] = true
				}
				if v := e.PayloadExtraction.HTTP.Sqlpp.EndpointPatterns; len(v) > 0 {
					m3["endpoint_patterns"] = v
				}
				if len(m3) > 0 {
					m2["sqlpp"] = m3
				}
			}
			if len(m2) > 0 {
				m1["http"] = m2
			}
		}
		if len(m1) > 0 {
			m["payload_extraction"] = m1
		}
	}
	if v := e.PostgresPreparedStatementsCacheSize; v != 0 {
		m["postgres_prepared_statements_cache_size"] = v
	}
	if e.ProtocolDebug {
		m["protocol_debug_print"] = true
	}
	{
		m1 := make(map[string]any)
		if e.RedisDbCache.Enabled {
			m1["enabled"] = true
		}
		if v := e.RedisDbCache.MaxSize; v != 0 {
			m1["max_size"] = v
		}
		if len(m1) > 0 {
			m["redis_db_cache"] = m1
		}
	}
	if v := e.StatsWakeupDataBytes; v != 0 {
		m["stats_wakeup_data_bytes"] = v
	}
	if e.TrackRequestHeaders {
		m["track_request_headers"] = true
	}
	if v := e.TrafficControlBackend; v != "" {
		m["traffic_control_backend"] = v
	}
	if v := e.WakeupLen; v != 0 {
		m["wakeup_len"] = v
	}
	return m
}

// Convert builds the injector configuration.
func (i Injector) Convert() map[string]any {
	m := make(map[string]any)
	if v := i.EnabledSdks; len(v) > 0 {
		m["enabled_sdks"] = v
	}
	if len(i.ExcludeInstrument) > 0 {
		m["exclude_instrument"] = i.ExcludeInstrument.Convert()
	}
	if v := i.ExporterOtlpEndpoint; v != "" {
		m["exporter_otlp_endpoint"] = v
	}
	if v := i.ExporterOtlpProtocol; v != "" {
		m["exporter_otlp_protocol"] = v
	}
	if v := i.ImageVersion; v != "" {
		m["image_version"] = v
	}
	if len(i.Instrument) > 0 {
		m["instrument"] = i.Instrument.Convert()
	}
	{
		m1 := make(map[string]any)
		if v := i.OtelExportedSignals.Logs; v != nil {
			m1["logs"] = *v
		}
		if v := i.OtelExportedSignals.Metrics; v != nil {
			m1["metrics"] = *v
		}
		if v := i.OtelExportedSignals.Traces; v != nil {
			m1["traces"] = *v
		}
		if len(m1) > 0 {
			m["otel_exported_signals"] = m1
		}
	}
	{
		m1 := make(map[string]any)
		if i.Resources.AddK8sIpAttribute {
			m1["add_k8s_ip_attribute"] = true
		}
		if i.Resources.AddK8sUidAttributes {
			m1["add_k8s_uid_attributes"] = true
		}
		if v := i.Resources.Attributes; len(v) > 0 {
			m1["attributes"] = v
		}
		if i.Resources.UseK8sLabelsForResourceAttributes {
			m1["use_k8s_labels_for_resource_attributes"] = true
		}
		if len(m1) > 0 {
			m["resources"] = m1
		}
	}
	if v := i.TracePropagators; len(v) > 0 {
		m["trace_propagators"] = v
	}
	{
		m1 := make(map[string]any)
		if v := i.TraceSampler.Arg; v != "" {
			m1["arg"] = v
		}
		if v := i.TraceSampler.Name; v != "" {
			m1["name"] = v
		}
		if len(m1) > 0 {
			m["trace_sampler"] = m1
		}
	}
	{
		m1 := make(map[string]any)
		if v := i.Webhook.ExternalDeploymentName; v != "" {
			m1["external_deployment_name"] = v
		}
		if len(m1) > 0 {
			m["webhook"] = m1
		}
	}
	if i.DisableAutoRestart {
		m["disable_auto_restart"] = true
	}
	return m
}

// Convert builds the javaagent configuration.
func (j Javaagent) Convert() map[string]any {
	m := make(map[string]any)
	if v := j.AttachTimeout; v != 0 {
		m["attach_timeout"] = v.String()
	}
	if j.Debug {
		m["debug"] = true
	}
	if j.DebugInstrumentation {
		m["debug_instrumentation"] = true
	}
	if v := j.Enabled; v != nil {
		m["enabled"] = *v
	}
	return m
}

// Convert builds the jvm_runtime_metrics configuration.
func (j JVMRuntimeMetrics) Convert() map[string]any {
	m := make(map[string]any)
	if j.Enabled {
		m["enabled"] = true
	}
	if v := j.SamplingInterval; v != 0 {
		m["sampling_interval"] = v.String()
	}
	return m
}

// Convert builds the nodejs configuration.
func (n Nodejs) Convert() map[string]any {
	m := make(map[string]any)
	if v := n.Enabled; v != nil {
		m["enabled"] = *v
	}
	return m
}

// Convert builds the routes configuration.
func (r Routes) Convert() map[string]any {
	m := make(map[string]any)
	if v := r.IgnoredEvents; v != "" {
		m["ignore_mode"] = v
	}
	if v := r.IgnorePatterns; len(v) > 0 {
		m["ignored_patterns"] = v
	}
	if v := r.MaxPathSegmentCardinality; v != 0 {
		m["max_path_segment_cardinality"] = v
	}
	if v := r.Patterns; len(v) > 0 {
		m["patterns"] = v
	}
	if v := r.Unmatch; v != "" {
		m["unmatched"] = v
	}
	if v := r.WildcardChar; v != "" {
		m["wildcard_char"] = v
	}
	return m
}

// Convert builds the stats configuration.
func (s Stats) Convert() map[string]any {
	m := make(map[string]any)
	if v := s.AgentIP; v != "" {
		m["agent_ip"] = v
	}
	if v := s.AgentIPIface; v != "" {
		m["agent_ip_iface"] = v
	}
	if v := s.AgentIPType; v != "" {
		m["agent_ip_type"] = v
	}
	if v := s.CIDRs; len(v) > 0 {
		m["cidrs"] = v
	}
	{
		m1 := make(map[string]any)
		if v := s.GeoIp.CacheExpiry; v != 0 {
			m1["cache_expiry"] = v.String()
		}
		if v := s.GeoIp.CacheLen; v != 0 {
			m1["cache_len"] = v
		}
		{
			m2 := make(map[string]any)
			if v := s.GeoIp.Ipinfo.Path; v != "" {
				m2["path"] = v
			}
			if len(m2) > 0 {
				m1["ipinfo"] = m2
			}
		}
		{
			m2 := make(map[string]any)
			if v := s.GeoIp.Maxmind.AsnPath; v != "" {
				m2["asn_path"] = v
			}
			if v := s.GeoIp.Maxmind.CountryPath; v != "" {
				m2["country_path"] = v
			}
			if len(m2) > 0 {
				m1["maxmind"] = m2
			}
		}
		if len(m1) > 0 {
			m["geo_ip"] = m1
		}
	}
	if s.Print {
		m["print_stats"] = true
	}
	{
		m1 := make(map[string]any)
		if v := s.ReverseDns.CacheExpiry; v != 0 {
			m1["cache_expiry"] = v.String()
		}
		if v := s.ReverseDns.CacheLen; v != 0 {
			m1["cache_len"] = v
		}
		if v := s.ReverseDns.Type; v != "" {
			m1["type"] = v
		}
		if len(m1) > 0 {
			m["reverse_dns"] = m1
		}
	}
	return m
}

// Convert builds the network configuration (metrics.network fields).
func (n Network) Convert() map[string]any {
	m := make(map[string]any)
	if v := n.AgentIP; v != "" {
		m["agent_ip"] = v
	}
	if v := n.AgentIPIface; v != "" {
		m["agent_ip_iface"] = v
	}
	if v := n.AgentIPType; v != "" {
		m["agent_ip_type"] = v
	}
	if v := n.CacheActiveTimeout; v != 0 {
		m["cache_active_timeout"] = v.String()
	}
	if v := n.CacheMaxFlows; v != 0 {
		m["cache_max_flows"] = v
	}
	if v := n.CIDRs; len(v) > 0 {
		m["cidrs"] = v
	}
	if v := n.Deduper; v != "" {
		m["deduper"] = v
	}
	if v := n.DeduperFCTTL; v != 0 {
		m["deduper_fc_ttl"] = v.String()
	}
	if v := n.Direction; v != "" {
		m["direction"] = v
	}
	if n.Enable {
		m["enable"] = true
	}
	if v := n.ExcludeInterfaces; len(v) > 0 {
		m["exclude_interfaces"] = v
	}
	if v := n.ExcludeProtocols; len(v) > 0 {
		m["exclude_protocols"] = v
	}
	{
		m1 := make(map[string]any)
		if v := n.GeoIp.CacheExpiry; v != 0 {
			m1["cache_expiry"] = v.String()
		}
		if v := n.GeoIp.CacheLen; v != 0 {
			m1["cache_len"] = v
		}
		{
			m2 := make(map[string]any)
			if v := n.GeoIp.Ipinfo.Path; v != "" {
				m2["path"] = v
			}
			if len(m2) > 0 {
				m1["ipinfo"] = m2
			}
		}
		{
			m2 := make(map[string]any)
			if v := n.GeoIp.Maxmind.AsnPath; v != "" {
				m2["asn_path"] = v
			}
			if v := n.GeoIp.Maxmind.CountryPath; v != "" {
				m2["country_path"] = v
			}
			if len(m2) > 0 {
				m1["maxmind"] = m2
			}
		}
		if len(m1) > 0 {
			m["geo_ip"] = m1
		}
	}
	if v := n.GuessPorts; v != "" {
		m["guess_ports"] = v
	}
	if v := n.Interfaces; len(v) > 0 {
		m["interfaces"] = v
	}
	if v := n.ListenInterfaces; v != "" {
		m["listen_interfaces"] = v
	}
	if v := n.ListenPollPeriod; v != 0 {
		m["listen_poll_period"] = v.String()
	}
	if n.PrintFlows {
		m["print_flows"] = true
	}
	if v := n.Protocols; len(v) > 0 {
		m["protocols"] = v
	}
	{
		m1 := make(map[string]any)
		if v := n.ReverseDns.CacheExpiry; v != 0 {
			m1["cache_expiry"] = v.String()
		}
		if v := n.ReverseDns.CacheLen; v != 0 {
			m1["cache_len"] = v
		}
		if v := n.ReverseDns.Type; v != "" {
			m1["type"] = v
		}
		if len(m1) > 0 {
			m["reverse_dns"] = m1
		}
	}
	if v := n.Sampling; v != 0 {
		m["sampling"] = v
	}
	if v := n.Source; v != "" {
		m["source"] = v
	}
	return m
}

// Convert builds a YAML map keyed by the "attr" field.
func (fs AttributeFamilies) Convert() map[string]any {
	m := make(map[string]any)
	for _, item := range fs {
		sub := make(map[string]any)
		if v := item.Equals; v != nil {
			sub["equals"] = *v
		}
		if v := item.GreaterEquals; v != nil {
			sub["greater_equals"] = *v
		}
		if v := item.GreaterThan; v != nil {
			sub["greater_than"] = *v
		}
		if v := item.LessEquals; v != nil {
			sub["less_equals"] = *v
		}
		if v := item.LessThan; v != nil {
			sub["less_than"] = *v
		}
		if v := item.Match; v != "" {
			sub["match"] = v
		}
		if v := item.NotEquals; v != nil {
			sub["not_equals"] = *v
		}
		if v := item.NotMatch; v != "" {
			sub["not_match"] = v
		}
		if len(sub) > 0 {
			m[item.Attr] = sub
		}
	}
	return m
}

// Convert builds a YAML map keyed by the "attr" field.
func (s Selections) Convert() map[string]any {
	m := make(map[string]any)
	for _, item := range s {
		sub := make(map[string]any)
		if v := item.Exclude; len(v) > 0 {
			sub["exclude"] = v
		}
		if v := item.Include; len(v) > 0 {
			sub["include"] = v
		}
		if len(sub) > 0 {
			m[item.Section] = sub
		}
	}
	return m
}

// Convert builds the services list YAML.
func (s Services) Convert() []map[string]any {
	result := make([]map[string]any, 0, len(s))
	for _, item := range s {
		m := make(map[string]any)
		if v := item.CmdArgs; v != "" {
			m["cmd_args"] = v
		}
		if item.ContainersOnly {
			m["containers_only"] = true
		}
		if v := item.Path; v != "" {
			m["exe_path"] = v
		}
		if v := item.ExportModes; len(v) > 0 {
			m["exports"] = v
		}
		if v := item.Kubernetes.DaemonSetName; v != "" {
			m["k8s_daemonset_name"] = v
		}
		if v := item.Kubernetes.DeploymentName; v != "" {
			m["k8s_deployment_name"] = v
		}
		if v := item.Kubernetes.Namespace; v != "" {
			m["k8s_namespace"] = v
		}
		if v := item.Kubernetes.OwnerName; v != "" {
			m["k8s_owner_name"] = v
		}
		if v := item.Kubernetes.PodAnnotations; len(v) > 0 {
			m["k8s_pod_annotations"] = v
		}
		if v := item.Kubernetes.PodLabels; len(v) > 0 {
			m["k8s_pod_labels"] = v
		}
		if v := item.Kubernetes.PodName; v != "" {
			m["k8s_pod_name"] = v
		}
		if v := item.Kubernetes.ReplicaSetName; v != "" {
			m["k8s_replicaset_name"] = v
		}
		if v := item.Kubernetes.StatefulSetName; v != "" {
			m["k8s_statefulset_name"] = v
		}
		if v := item.Languages; v != "" {
			m["languages"] = v
		}
		if v := item.Name; v != "" {
			m["name"] = v
		}
		if v := item.Namespace; v != "" {
			m["namespace"] = v
		}
		if v := item.OpenPorts; v != "" {
			m["open_ports"] = v
		}
		{
			m1 := make(map[string]any)
			if v := item.Sampler.Arg; v != "" {
				m1["arg"] = v
			}
			if v := item.Sampler.Name; v != "" {
				m1["name"] = v
			}
			if len(m1) > 0 {
				m["sampler"] = m1
			}
		}
		if len(m) > 0 {
			result = append(result, m)
		}
	}
	return result
}
