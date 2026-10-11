package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Samplers that take a ratio arg — a semantic rule the schema enum can't express.
// The accepted name set lives in validation_enums.go (samplerNameValues).
const (
	SamplerTraceIDRatio            = "traceidratio"
	SamplerParentBasedTraceIDRatio = "parentbased_traceidratio"
)

// Validate validates the SamplerConfig
func (args SamplerConfig) Validate() error {
	if args.Name == "" {
		return nil // Empty name is valid, will use default
	}

	if !validSamplerName(args.Name) {
		return fmt.Errorf("invalid sampler name %q. Valid values are: %s", args.Name, strings.Join(samplerNameValues, ", "))
	}

	// Validate arg for ratio-based samplers
	if args.Name == SamplerTraceIDRatio || args.Name == SamplerParentBasedTraceIDRatio {
		if args.Arg == "" {
			return fmt.Errorf("sampler %q requires an arg parameter with a ratio value between 0 and 1", args.Name)
		}

		ratio, err := strconv.ParseFloat(args.Arg, 64)
		if err != nil {
			return fmt.Errorf("invalid arg %q for sampler %q: must be a valid decimal number", args.Arg, args.Name)
		}

		if ratio < 0 || ratio > 1 {
			return fmt.Errorf("invalid arg %q for sampler %q: ratio must be between 0 and 1 (inclusive)", args.Arg, args.Name)
		}
	}

	return nil
}

// hasNetworkFeature checks if network feature is enabled in metrics. "*" and "all"
// enable every feature family, network included.
func (args Metrics) hasNetworkFeature() bool {
	for _, feature := range args.Features {
		switch feature {
		case "network", "*", "all":
			return true
		}
	}
	return false
}

// hasAppFeature checks if any application feature is enabled in metrics. "*" and
// "all" enable every feature family, application included.
func (args Metrics) hasAppFeature() bool {
	for _, feature := range args.Features {
		switch feature {
		case "application", "application_red", "application_sizes", "application_host", "application_span", "application_service_graph",
			"application_process", "application_span_otel", "application_span_sizes", "*", "all":
			return true
		}
	}
	return false
}

// Validate validates the Metrics configuration
func (args Metrics) Validate() error {
	for _, instrumentation := range args.Instrumentations {
		if !validInstrumentation(instrumentation) {
			return fmt.Errorf("metrics.instrumentations: invalid value %q", instrumentation)
		}
	}

	for _, feature := range args.Features {
		if !validMetricFeature(feature) {
			return fmt.Errorf("metrics.features: invalid value %q", feature)
		}
	}
	if slices.Contains(args.Features, "application_sizes") &&
		!slices.Contains(args.Features, "application_red") &&
		!slices.Contains(args.Features, "application") &&
		!slices.Contains(args.Features, "*") && !slices.Contains(args.Features, "all") {

		return fmt.Errorf("metrics.features: application_sizes requires application or application_red in the same features list")
	}
	return nil
}

// Validate validates the name resolution settings. Zero values are omitted so
// Beyla uses its defaults.
func (args NameResolver) Validate() error {
	for _, source := range args.Sources {
		if !slices.Contains(nameResolverSourceValues, source) {
			return fmt.Errorf("name_resolver.sources: invalid value %q", source)
		}
	}
	if args.CacheLen < 0 {
		return fmt.Errorf("name_resolver.cache_len must be greater than 0 when set")
	}
	if args.CacheExpiry < 0 {
		return fmt.Errorf("name_resolver.cache_expiry must be greater than 0 when set")
	}
	if args.ECS.RefreshInterval < 0 {
		return fmt.Errorf("name_resolver.ecs.refresh_interval must be greater than 0 when set")
	}
	return nil
}

// Validate validates the .NET runtime collection settings.
func (args DotnetRuntimeMetrics) Validate() error {
	if args.SamplingInterval < 0 {
		return fmt.Errorf("dotnet_runtime_metrics.sampling_interval must be greater than 0 when set")
	}
	if args.Timeout < 0 {
		return fmt.Errorf("dotnet_runtime_metrics.timeout must be greater than 0 when set")
	}
	return nil
}

// Validate validates the Services configuration
func (args Services) Validate() error {
	for i, svc := range args {
		// Check if any Kubernetes fields are defined
		hasKubernetes := svc.Kubernetes.Namespace != "" ||
			svc.Kubernetes.PodName != "" ||
			svc.Kubernetes.DeploymentName != "" ||
			svc.Kubernetes.ReplicaSetName != "" ||
			svc.Kubernetes.StatefulSetName != "" ||
			svc.Kubernetes.DaemonSetName != "" ||
			svc.Kubernetes.OwnerName != "" ||
			len(svc.Kubernetes.PodLabels) > 0

		if svc.OpenPorts == "" && svc.Path == "" && svc.CmdArgs == "" && !hasKubernetes {
			return fmt.Errorf("discovery.services[%d] must define at least one of: open_ports, exe_path, cmd_args, or kubernetes configuration", i)
		}
	}
	return nil
}

// Validate validates the Arguments configuration
func (args *Arguments) Validate() error {
	hasAppFeature := args.Metrics.hasAppFeature()

	if args.TracePrinter == "" {
		args.TracePrinter = "disabled"
	} else if !validTracePrinter(args.TracePrinter) {
		return fmt.Errorf("trace_printer: invalid value %q. Valid values are: %s", args.TracePrinter, strings.Join(tracePrinterValues, ", "))
	}

	if err := args.Metrics.Validate(); err != nil {
		return err
	}

	if err := args.Traces.Validate(); err != nil {
		return err
	}
	if err := args.NameResolver.Validate(); err != nil {
		return err
	}
	if err := args.DotnetRuntimeMetrics.Validate(); err != nil {
		return err
	}
	if args.EBPF.KafkaConsumerGroupCacheSize < 0 {
		return fmt.Errorf("ebpf.kafka_consumer_group_cache_size must be greater than 0 when set")
	}
	if args.EBPF.KafkaConsumerGroupTTL < 0 {
		return fmt.Errorf("ebpf.kafka_consumer_group_ttl must be greater than 0 when set")
	}
	if v := args.EBPF.GoHTTPClientBufferTimeout; v != nil && *v < 0 {
		return fmt.Errorf("ebpf.go_http_client_buffer_timeout must be greater than or equal to 0")
	}

	// If traces block is defined with instrumentations, output section must be defined
	if len(args.Traces.Instrumentations) > 0 || args.Traces.Sampler.Name != "" {
		if args.Output == nil {
			return fmt.Errorf("traces block is defined but output section is missing. When using traces configuration, you must define an output block")
		}
	}

	if hasAppFeature {
		// Check if any discovery method is configured (new or legacy)
		hasAnyDiscovery := len(args.Discovery.Services) > 0 ||
			len(args.Discovery.Survey) > 0 ||
			len(args.Discovery.Instrument) > 0

		if !hasAnyDiscovery {
			return fmt.Errorf("discovery.services, discovery.instrument, or discovery.survey is required when application features are enabled")
		}

		// Validate legacy services field
		if len(args.Discovery.Services) > 0 {
			if err := args.Discovery.Services.Validate(); err != nil {
				return fmt.Errorf("invalid discovery configuration: %s", err.Error())
			}
		}

		// Validate survey field
		if len(args.Discovery.Survey) > 0 {
			if err := args.Discovery.Survey.Validate(); err != nil {
				return fmt.Errorf("invalid survey configuration: %s", err.Error())
			}
		}

		// Validate new instrument field
		if len(args.Discovery.Instrument) > 0 {
			if err := args.Discovery.Instrument.Validate(); err != nil {
				return fmt.Errorf("invalid instrument configuration: %s", err.Error())
			}
		}
	}

	// Validate legacy exclude_services field
	if len(args.Discovery.ExcludeServices) > 0 {
		if err := args.Discovery.ExcludeServices.Validate(); err != nil {
			return fmt.Errorf("invalid exclude_services configuration: %s", err.Error())
		}
	}

	// Validate new exclude_instrument field
	if len(args.Discovery.ExcludeInstrument) > 0 {
		if err := args.Discovery.ExcludeInstrument.Validate(); err != nil {
			return fmt.Errorf("invalid exclude_instrument configuration: %s", err.Error())
		}
	}

	// Validate new default_exclude_instrument field
	if len(args.Discovery.DefaultExcludeInstrument) > 0 {
		if err := args.Discovery.DefaultExcludeInstrument.Validate(); err != nil {
			return fmt.Errorf("invalid default_exclude_instrument configuration: %s", err.Error())
		}
	}

	// Validate per-service samplers for legacy services
	for i, service := range args.Discovery.Services {
		if err := service.Sampler.Validate(); err != nil {
			return fmt.Errorf("invalid sampler configuration in discovery.services[%d]: %s", i, err.Error())
		}
	}

	// Validate per-service samplers for new instrument field
	for i, service := range args.Discovery.Instrument {
		if err := service.Sampler.Validate(); err != nil {
			return fmt.Errorf("invalid sampler configuration in discovery.instrument[%d]: %s", i, err.Error())
		}
	}

	// Validate per-service samplers for survey field
	for i, service := range args.Discovery.Survey {
		if err := service.Sampler.Validate(); err != nil {
			return fmt.Errorf("invalid sampler configuration in discovery.survey[%d]: %s", i, err.Error())
		}
	}

	if args.InternalMetrics.Exporter == "otel" && (args.Output == nil || len(args.Output.Metrics) == 0) {
		return fmt.Errorf("internal_metrics.exporter = \"otel\" requires output.metrics to be configured")
	}

	return nil
}

// Validate validates the Traces configuration
func (args Traces) Validate() error {
	for _, instrumentation := range args.Instrumentations {
		if !validInstrumentation(instrumentation) {
			return fmt.Errorf("traces.instrumentations: invalid value %q", instrumentation)
		}
	}

	// Validate the global sampler config
	if err := args.Sampler.Validate(); err != nil {
		return fmt.Errorf("invalid global sampler configuration: %s", err.Error())
	}

	return nil
}
