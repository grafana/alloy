// Package googlecloudconfig contains configuration arguments specific to the Google Cloud exporter.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/exporter/googlecloudexporter/README.md
// for documentation.
package googlecloudconfig

import (
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"github.com/samber/lo"
)

type GoogleCloudImpersonateArguments struct {
	TargetPrincipal string   `alloy:"target_principal,attr,optional"`
	Subject         string   `alloy:"subject,attr,optional"`
	Delegates       []string `alloy:"delegates,attr,optional"`
}

func (args *GoogleCloudImpersonateArguments) Convert(c collector.ImpersonateConfig) collector.ImpersonateConfig {
	c.TargetPrincipal = args.TargetPrincipal
	c.Subject = args.Subject
	c.Delegates = args.Delegates
	return c
}

func (args *GoogleCloudImpersonateArguments) SetToDefault() {
	*args = GoogleCloudImpersonateArguments{}
}

type GoogleCloudMetricArguments struct {
	Prefix                           string           `alloy:"prefix,attr,optional"`
	Endpoint                         string           `alloy:"endpoint,attr,optional"`
	Compression                      string           `alloy:"compression,attr,optional"`
	GRPCPoolSize                     int              `alloy:"grpc_pool_size,attr,optional"`
	UseInsecure                      bool             `alloy:"use_insecure,attr,optional"`
	KnownDomains                     []string         `alloy:"known_domains,attr,optional"`
	SkipCreateDescriptor             bool             `alloy:"skip_create_descriptor,attr,optional"`
	InstrumentationLibraryLabels     bool             `alloy:"instrumentation_library_labels,attr,optional"`
	CreateServiceTimeseries          bool             `alloy:"create_service_timeseries,attr,optional"`
	CreateMetricDescriptorBufferSize int              `alloy:"create_metric_descriptor_buffer_size,attr,optional"`
	ServiceResourceLabels            bool             `alloy:"service_resource_labels,attr,optional"`
	ResourceFilters                  []ResourceFilter `alloy:"resource_filters,attr,optional"`
	CumulativeNormalization          bool             `alloy:"cumulative_normalization,attr,optional"`
	SumOfSquaredDeviation            bool             `alloy:"sum_of_squared_deviation,attr,optional"`
	ExperimentalWAL                  *ExperimentalWAL `alloy:"experimental_wal,block,optional"`
}

type ResourceFilter struct {
	Prefix string `alloy:"prefix,attr,optional"`
	Regex  string `alloy:"regex,attr,optional"`
}

// convert is a helper function for lo.Map.
func (f ResourceFilter) convert(_ int) collector.ResourceFilter {
	return collector.ResourceFilter{
		Prefix: f.Prefix,
		Regex:  f.Regex,
	}
}

// ExperimentalWAL represents experimental_wal config.
//
// Here, we could default the `directory` argument to Alloy's storage.path config value.
// However, to maintain simplicity as a wrapper around the upstream contrib library and ensure predictable behavior for users,
// we decided to keep the configuration style consistent with the upstream library.
type ExperimentalWAL struct {
	Directory  string        `alloy:"directory,attr,optional"`
	MaxBackoff time.Duration `alloy:"max_backoff,attr,optional"`
}

func (e *ExperimentalWAL) convert() *collector.WALConfig {
	if e == nil {
		return nil
	}
	return &collector.WALConfig{
		Directory:  e.Directory,
		MaxBackoff: e.MaxBackoff,
	}
}

func (args *GoogleCloudMetricArguments) Convert(c collector.MetricConfig) collector.MetricConfig {
	c.Prefix = args.Prefix
	c.ClientConfig = collector.ClientConfig{
		Endpoint:     args.Endpoint,
		Compression:  args.Compression,
		UseInsecure:  args.UseInsecure,
		GRPCPoolSize: args.GRPCPoolSize,
	}
	c.KnownDomains = args.KnownDomains
	c.SkipCreateMetricDescriptor = args.SkipCreateDescriptor
	c.InstrumentationLibraryLabels = args.InstrumentationLibraryLabels
	c.CreateServiceTimeSeries = args.CreateServiceTimeseries
	c.CreateMetricDescriptorBufferSize = args.CreateMetricDescriptorBufferSize
	c.ServiceResourceLabels = args.ServiceResourceLabels
	c.ResourceFilters = lo.Map(args.ResourceFilters, ResourceFilter.convert)
	c.CumulativeNormalization = args.CumulativeNormalization
	c.EnableSumOfSquaredDeviation = args.SumOfSquaredDeviation
	c.WALConfig = args.ExperimentalWAL.convert()
	return c
}

func (args *GoogleCloudMetricArguments) SetToDefault() {
	*args = GoogleCloudMetricArguments{
		Endpoint:                         "monitoring.googleapis.com:443",
		KnownDomains:                     []string{"googleapis.com", "kubernetes.io", "istio.io", "knative.dev"},
		Prefix:                           "workload.googleapis.com",
		CreateMetricDescriptorBufferSize: 10,
		InstrumentationLibraryLabels:     true,
		ServiceResourceLabels:            true,
		CumulativeNormalization:          true,
	}
}

type GoogleCloudTraceArguments struct {
	Endpoint          string              `alloy:"endpoint,attr,optional"`
	GRPCPoolSize      int                 `alloy:"grpc_pool_size,attr,optional"`
	UseInsecure       bool                `alloy:"use_insecure,attr,optional"`
	AttributeMappings []AttributeMappings `alloy:"attribute_mappings,attr,optional"`
}

type AttributeMappings struct {
	Key         string `alloy:"key,attr"`
	Replacement string `alloy:"replacement,attr"`
}

// convert is a helper function for lo.Map.
func (a AttributeMappings) convert(_ int) collector.AttributeMapping {
	return collector.AttributeMapping{
		Key:         a.Key,
		Replacement: a.Replacement,
	}
}

func (args *GoogleCloudTraceArguments) Convert(c collector.TraceConfig) collector.TraceConfig {
	c.ClientConfig = collector.ClientConfig{
		Endpoint:     args.Endpoint,
		UseInsecure:  args.UseInsecure,
		GRPCPoolSize: args.GRPCPoolSize,
	}
	c.AttributeMappings = lo.Map(args.AttributeMappings, AttributeMappings.convert)
	return c
}

func (args *GoogleCloudTraceArguments) SetToDefault() {
	*args = GoogleCloudTraceArguments{
		Endpoint: "cloudtrace.googleapis.com:443",
	}
}

type GoogleCloudLogArguments struct {
	Endpoint              string           `alloy:"endpoint,attr,optional"`
	Compression           string           `alloy:"compression,attr,optional"`
	GRPCPoolSize          int              `alloy:"grpc_pool_size,attr,optional"`
	UseInsecure           bool             `alloy:"use_insecure,attr,optional"`
	DefaultLogName        string           `alloy:"default_log_name,attr,optional"`
	ResourceFilters       []ResourceFilter `alloy:"resource_filters,attr,optional"`
	ServiceResourceLabels bool             `alloy:"service_resource_labels,attr,optional"`
	ErrorReportingType    bool             `alloy:"error_reporting_type,attr,optional"`
}

func (args *GoogleCloudLogArguments) Convert(c collector.LogConfig) collector.LogConfig {
	c.DefaultLogName = args.DefaultLogName
	c.ResourceFilters = lo.Map(args.ResourceFilters, ResourceFilter.convert)
	c.ClientConfig = collector.ClientConfig{
		Endpoint:     args.Endpoint,
		Compression:  args.Compression,
		UseInsecure:  args.UseInsecure,
		GRPCPoolSize: args.GRPCPoolSize,
	}
	c.ServiceResourceLabels = args.ServiceResourceLabels
	c.ErrorReportingType = args.ErrorReportingType
	return c
}

func (args *GoogleCloudLogArguments) SetToDefault() {
	*args = GoogleCloudLogArguments{
		Endpoint:              "logging.googleapis.com:443",
		ServiceResourceLabels: true,
	}
}
