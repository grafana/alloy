//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"fmt"
	"strconv"
)

// isValidInstrumentation checks if an instrumentation type is valid
func isValidInstrumentation(instrumentation string) bool {
	switch instrumentation {
	case "*", "http", "grpc", "redis", "kafka", "sql", "gpu", "mongo":
		return true
	default:
		return false
	}
}

// Validate validates the SamplerConfig
func (args SamplerConfig) Validate() error {
	if args.Name == "" {
		return nil // Empty name is valid, will use default
	}

	validSamplers := map[string]bool{
		SamplerAlwaysOn:                true,
		SamplerAlwaysOff:               true,
		SamplerTraceIDRatio:            true,
		SamplerParentBasedAlwaysOn:     true,
		SamplerParentBasedAlwaysOff:    true,
		SamplerParentBasedTraceIDRatio: true,
	}

	if !validSamplers[args.Name] {
		return fmt.Errorf("invalid sampler name %q. Valid values are: %s, %s, %s, %s, %s, %s", args.Name,
			SamplerAlwaysOn, SamplerAlwaysOff, SamplerTraceIDRatio,
			SamplerParentBasedAlwaysOn, SamplerParentBasedAlwaysOff, SamplerParentBasedTraceIDRatio)
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

// hasNetworkFeature checks if network feature is enabled in metrics
func (args Metrics) hasNetworkFeature() bool {
	for _, feature := range args.Features {
		if feature == "network" {
			return true
		}
	}
	return false
}

// hasAppFeature checks if any application feature is enabled in metrics
func (args Metrics) hasAppFeature() bool {
	for _, feature := range args.Features {
		switch feature {
		case "application", "application_host", "application_span", "application_service_graph",
			"application_process", "application_span_otel", "application_span_sizes":
			return true
		}
	}
	return false
}

// Validate validates the Metrics configuration
func (args Metrics) Validate() error {
	for _, instrumentation := range args.Instrumentations {
		if !isValidInstrumentation(instrumentation) {
			return fmt.Errorf("metrics.instrumentations: invalid value %q", instrumentation)
		}
	}

	validFeatures := map[string]struct{}{
		"application": {}, "application_span": {}, "application_span_otel": {},
		"application_span_sizes": {}, "application_host": {},
		"application_service_graph": {}, "application_process": {},
		"network": {}, "network_inter_zone": {},
	}
	for _, feature := range args.Features {
		if _, ok := validFeatures[feature]; !ok {
			return fmt.Errorf("metrics.features: invalid value %q", feature)
		}
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

		if svc.OpenPorts == "" && svc.Path == "" && !hasKubernetes {
			return fmt.Errorf("discovery.services[%d] must define at least one of: open_ports, exe_path, or kubernetes configuration", i)
		}
	}
	return nil
}

// Validate validates the Arguments configuration
func (args *Arguments) Validate() error {
	hasAppFeature := args.Metrics.hasAppFeature()

	if args.TracePrinter == "" {
		args.TracePrinter = "disabled"
	} else {
		validPrinters := map[string]bool{
			"disabled": true, "counter": true, "text": true, "json": true, "json_indent": true,
		}
		if !validPrinters[args.TracePrinter] {
			return fmt.Errorf("trace_printer: invalid value %q. Valid values are: disabled, counter, text, json, json_indent", args.TracePrinter)
		}
	}

	if err := args.Metrics.Validate(); err != nil {
		return err
	}

	if err := args.Traces.Validate(); err != nil {
		return err
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
		if !isValidInstrumentation(instrumentation) {
			return fmt.Errorf("traces.instrumentations: invalid value %q", instrumentation)
		}
	}

	// Validate the global sampler config
	if err := args.Sampler.Validate(); err != nil {
		return fmt.Errorf("invalid global sampler configuration: %s", err.Error())
	}

	return nil
}
