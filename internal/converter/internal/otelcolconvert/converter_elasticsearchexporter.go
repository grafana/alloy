package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/elasticsearch/config"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
)

func init() {
	converters = append(converters, elasticsearchExporterConverter{})
}

type elasticsearchExporterConverter struct{}

func (elasticsearchExporterConverter) Factory() component.Factory {
	return elasticsearchexporter.NewFactory()
}

func (elasticsearchExporterConverter) InputComponentName() string {
	return "otelcol.exporter.elasticsearch"
}

func (elasticsearchExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	args := toElasticsearchExporter(cfg.(*elasticsearchexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "elasticsearch"}, label, args, common.GetAlloyTypesOverrideHook())

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toElasticsearchExporter(cfg *elasticsearchexporter.Config) *config.ElasticsearchArguments {
	httpClient := toHTTPClientArguments(cfg.ClientConfig)
	return &config.ElasticsearchArguments{
		Endpoints: cfg.Endpoints,
		CloudID:   cfg.CloudID,

		NumWorkers: cfg.NumWorkers,

		LogsIndex:    cfg.LogsIndex,
		MetricsIndex: cfg.MetricsIndex,
		TracesIndex:  cfg.TracesIndex,

		Pipeline: cfg.Pipeline,

		IncludeSourceOnError: cfg.IncludeSourceOnError,
		MetadataKeys:         cfg.MetadataKeys,

		Client: toElasticsearchClientArguments(httpClient),
		Authentication: config.AuthenticationArguments{
			User:     cfg.Authentication.User,
			Password: alloytypes.Secret(cfg.Authentication.Password.String()),
			APIKey:   alloytypes.Secret(cfg.Authentication.APIKey.String()),
		},
		Discovery: config.DiscoveryArguments{
			OnStart:  cfg.Discovery.OnStart,
			Interval: cfg.Discovery.Interval,
		},
		Retry: config.RetryArguments{
			Enabled:         cfg.Retry.Enabled,
			MaxRetries:      cfg.Retry.MaxRetries,
			InitialInterval: cfg.Retry.InitialInterval,
			MaxInterval:     cfg.Retry.MaxInterval,
			RetryOnStatus:   cfg.Retry.RetryOnStatus,
		},
		Flush: config.FlushArguments{
			Bytes:    cfg.Flush.Bytes,
			Interval: cfg.Flush.Interval,
		},
		Mapping: config.MappingArguments{
			Mode:         cfg.Mapping.Mode,
			AllowedModes: cfg.Mapping.AllowedModes,
		},
		LogstashFormat: config.LogstashFormatArguments{
			Enabled:         cfg.LogstashFormat.Enabled,
			PrefixSeparator: cfg.LogstashFormat.PrefixSeparator,
			DateFormat:      cfg.LogstashFormat.DateFormat,
		},
		Telemetry: config.TelemetryArguments{
			LogRequestBody:              cfg.LogRequestBody,
			LogResponseBody:             cfg.LogResponseBody,
			LogFailedDocsInput:          cfg.LogFailedDocsInput,
			LogFailedDocsInputRateLimit: cfg.LogFailedDocsInputRateLimit,
		},

		LogsDynamicIndex:    config.DynamicIndexArguments{Enabled: cfg.LogsDynamicIndex.Enabled},
		MetricsDynamicIndex: config.DynamicIndexArguments{Enabled: cfg.MetricsDynamicIndex.Enabled},
		TracesDynamicIndex:  config.DynamicIndexArguments{Enabled: cfg.TracesDynamicIndex.Enabled},
		LogsDynamicID:       config.DynamicIDArguments{Enabled: cfg.LogsDynamicID.Enabled},
		TracesDynamicID:     config.DynamicIDArguments{Enabled: cfg.TracesDynamicID.Enabled},
		LogsDynamicPipeline: config.DynamicPipelineArguments{Enabled: cfg.LogsDynamicPipeline.Enabled},

		SendingQueue: toQueueArguments(cfg.QueueBatchConfig),
		DebugMetrics: common.DefaultValue[config.ElasticsearchArguments]().DebugMetrics,
	}
}

func toElasticsearchClientArguments(hc otelcol.HTTPClientArguments) config.ClientArguments {
	return config.ClientArguments{
		Endpoint:             hc.Endpoint,
		ProxyURL:             hc.ProxyUrl,
		Compression:          hc.Compression,
		CompressionParams:    hc.CompressionParams,
		TLS:                  hc.TLS,
		ReadBufferSize:       hc.ReadBufferSize,
		WriteBufferSize:      hc.WriteBufferSize,
		Timeout:              hc.Timeout,
		Headers:              hc.Headers,
		MaxIdleConns:         hc.MaxIdleConns,
		MaxIdleConnsPerHost:  hc.MaxIdleConnsPerHost,
		MaxConnsPerHost:      hc.MaxConnsPerHost,
		IdleConnTimeout:      hc.IdleConnTimeout,
		DisableKeepAlives:    hc.DisableKeepAlives,
		HTTP2ReadIdleTimeout: hc.HTTP2ReadIdleTimeout,
		HTTP2PingTimeout:     hc.HTTP2PingTimeout,
		ForceAttemptHTTP2:    hc.ForceAttemptHTTP2,
		Authentication:       hc.Authentication,
		Cookies:              hc.Cookies,
	}
}
