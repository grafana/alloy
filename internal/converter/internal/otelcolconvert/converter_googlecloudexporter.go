package otelcolconvert

import (
	"fmt"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter"
	"github.com/samber/lo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloud"
	googlecloudconfig "github.com/grafana/alloy/internal/component/otelcol/exporter/googlecloud/config"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
)

func init() {
	converters = append(converters, googleCloudExporterConverter{})
}

type googleCloudExporterConverter struct{}

func (googleCloudExporterConverter) Factory() component.Factory {
	return googlecloudexporter.NewFactory()
}

func (googleCloudExporterConverter) InputComponentName() string {
	return "otelcol.exporter.googlecloud"
}

func (googleCloudExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toGoogleCloudExporter(cfg.(*googlecloudexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "googlecloud"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toGoogleCloudExporter(cfg *googlecloudexporter.Config) *googlecloud.Arguments {
	return &googlecloud.Arguments{
		Queue:                   toQueueArguments(cfg.QueueSettings),
		Project:                 cfg.ProjectID,
		DestinationProjectQuota: cfg.DestinationProjectQuota,
		UserAgent:               cfg.UserAgent,
		Impersonate:             toGoogleCloudImpersonateArguments(cfg.ImpersonateConfig),
		Metric:                  toGoogleCloudMetricArguments(cfg.MetricConfig),
		Trace:                   toGoogleCloudTraceArguments(cfg.TraceConfig),
		Log:                     toGoogleCloudLogArguments(cfg.LogConfig),
		DebugMetrics:            common.DefaultValue[googlecloud.Arguments]().DebugMetrics,
	}
}

func toGoogleCloudImpersonateArguments(cfg collector.ImpersonateConfig) googlecloudconfig.GoogleCloudImpersonateArguments {
	return googlecloudconfig.GoogleCloudImpersonateArguments{
		TargetPrincipal: cfg.TargetPrincipal,
		Subject:         cfg.Subject,
		Delegates:       cfg.Delegates,
	}
}

func toGoogleCloudMetricArguments(cfg collector.MetricConfig) googlecloudconfig.GoogleCloudMetricArguments {
	return googlecloudconfig.GoogleCloudMetricArguments{
		Prefix:                           cfg.Prefix,
		Endpoint:                         cfg.ClientConfig.Endpoint,
		Compression:                      cfg.ClientConfig.Compression,
		GRPCPoolSize:                     cfg.ClientConfig.GRPCPoolSize,
		UseInsecure:                      cfg.ClientConfig.UseInsecure,
		KnownDomains:                     cfg.KnownDomains,
		SkipCreateDescriptor:             cfg.SkipCreateMetricDescriptor,
		InstrumentationLibraryLabels:     cfg.InstrumentationLibraryLabels,
		CreateServiceTimeseries:          cfg.CreateServiceTimeSeries,
		CreateMetricDescriptorBufferSize: cfg.CreateMetricDescriptorBufferSize,
		ServiceResourceLabels:            cfg.ServiceResourceLabels,
		ResourceFilters:                  lo.Map(cfg.ResourceFilters, toGoogleCloudResourceFilter),
		CumulativeNormalization:          cfg.CumulativeNormalization,
		SumOfSquaredDeviation:            cfg.EnableSumOfSquaredDeviation,
		ExperimentalWAL:                  toGoogleCloudExperimentalWAL(cfg.WALConfig),
	}
}

func toGoogleCloudResourceFilter(f collector.ResourceFilter, _ int) googlecloudconfig.ResourceFilter {
	return googlecloudconfig.ResourceFilter{
		Prefix: f.Prefix,
		Regex:  f.Regex,
	}
}

func toGoogleCloudExperimentalWAL(cfg *collector.WALConfig) *googlecloudconfig.ExperimentalWAL {
	if cfg == nil {
		return nil
	}
	return &googlecloudconfig.ExperimentalWAL{
		Directory:  cfg.Directory,
		MaxBackoff: cfg.MaxBackoff,
	}
}

func toGoogleCloudTraceArguments(cfg collector.TraceConfig) googlecloudconfig.GoogleCloudTraceArguments {
	return googlecloudconfig.GoogleCloudTraceArguments{
		Endpoint:          cfg.ClientConfig.Endpoint,
		GRPCPoolSize:      cfg.ClientConfig.GRPCPoolSize,
		UseInsecure:       cfg.ClientConfig.UseInsecure,
		AttributeMappings: lo.Map(cfg.AttributeMappings, toGoogleCloudAttributeMapping),
	}
}

func toGoogleCloudAttributeMapping(a collector.AttributeMapping, _ int) googlecloudconfig.AttributeMappings {
	return googlecloudconfig.AttributeMappings{
		Key:         a.Key,
		Replacement: a.Replacement,
	}
}

func toGoogleCloudLogArguments(cfg collector.LogConfig) googlecloudconfig.GoogleCloudLogArguments {
	return googlecloudconfig.GoogleCloudLogArguments{
		Endpoint:              cfg.ClientConfig.Endpoint,
		Compression:           cfg.ClientConfig.Compression,
		GRPCPoolSize:          cfg.ClientConfig.GRPCPoolSize,
		UseInsecure:           cfg.ClientConfig.UseInsecure,
		DefaultLogName:        cfg.DefaultLogName,
		ResourceFilters:       lo.Map(cfg.ResourceFilters, toGoogleCloudResourceFilter),
		ServiceResourceLabels: cfg.ServiceResourceLabels,
		ErrorReportingType:    cfg.ErrorReportingType,
	}
}
