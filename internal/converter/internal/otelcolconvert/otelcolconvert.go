package otelcolconvert

import (
	"bytes"
	"context"
	"fmt"

	"github.com/grafana/agent/internal/converter/diag"
	"github.com/grafana/agent/internal/converter/internal/common"
	"github.com/grafana/river/token/builder"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
)

// This package is split into a set of [componentConverter] implementations
// which convert a single OpenTelemetry Collector component into one or more
// Flow components.
//
// To support converting a new OpenTelmetry Component, follow these steps and
// replace COMPONENT with the name of the component being converted:
//
//   1. Create a file named "converter_COMPONENT.go".
//
//   2. Create a struct named "converterCOMPONENT" which implements the
// 		  [componentConverter] interface.
//
//   3. Add the following init function to the top of the file:
//
//      func init() {
//   	    addConverter(converterCOMPONENT{})
//      }

// Convert implements an Opentelemetry Collector config converter.
//
// For compatibility with other converters, the extraArgs paramater is defined
// but unused, and a critical error diagnostic is returned if extraArgs is
// non-empty.
func Convert(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(extraArgs) > 0 {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("extra arguments are not supported for the otelcol converter: %s", extraArgs))
		return nil, diags
	}

	cfg, err := readOpentelemetryConfig(in)
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, err.Error())
		return nil, diags
	}
	if err := cfg.Validate(); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to validate config: %s", err))
		return nil, diags
	}

	f := builder.NewFile()

	diags.AddAll(appendConfig(f, cfg))
	diags.AddAll(common.ValidateNodes(f))

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to render Flow config: %s", err.Error()))
		return nil, diags
	}

	if len(buf.Bytes()) == 0 {
		return nil, diags
	}

	prettyByte, newDiags := common.PrettyPrint(buf.Bytes())
	diags.AddAll(newDiags)
	return prettyByte, diags
}

func readOpentelemetryConfig(in []byte) (*otelcol.Config, error) {
	provider := yamlprovider.New()

	configProvider, err := otelcol.NewConfigProvider(otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs: []string{"yaml:" + string(in)},
			Providers: map[string]confmap.Provider{
				provider.Scheme(): provider,
			},
			Converters: []confmap.Converter{expandconverter.New()},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create otelcol config provider: %w", err)
	}

	cfg, err := configProvider.Get(context.Background(), getFactories())
	if err != nil {
		// TODO(rfratto): users may pass unknown components in YAML here. Can we
		// improve the errors? Can we ignore the errors?
		return nil, fmt.Errorf("failed to get otelcol config: %w", err)
	}

	return cfg, nil
}

func getFactories() otelcol.Factories {
	facts := otelcol.Factories{
		Receivers:  make(map[component.Type]receiver.Factory),
		Processors: make(map[component.Type]processor.Factory),
		Exporters:  make(map[component.Type]exporter.Factory),
		Extensions: make(map[component.Type]extension.Factory),
		Connectors: make(map[component.Type]connector.Factory),
	}

	for _, converter := range converters {
		fact := converter.Factory()

		switch fact := fact.(type) {
		case receiver.Factory:
			facts.Receivers[fact.Type()] = fact
		case processor.Factory:
			facts.Processors[fact.Type()] = fact
		case exporter.Factory:
			facts.Exporters[fact.Type()] = fact
		case extension.Factory:
			facts.Extensions[fact.Type()] = fact
		case connector.Factory:
			facts.Connectors[fact.Type()] = fact

		default:
			panic(fmt.Sprintf("unknown component factory type %T", fact))
		}
	}

	return facts
}

// appendConfig converts the provided OpenTelemetry config into an equivalent
// Flow config and appends the result to the provided file.
func appendConfig(file *builder.File, cfg *otelcol.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	groups, err := createPipelineGroups(cfg.Service.Pipelines)
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to interpret config: %s", err))
		return diags
	}

	// NOTE(rfratto): here, the same component ID will be instantiated once for
	// every group it's in. This means that converting receivers in multiple
	// groups will fail at runtime, as there will be two components attempting to
	// listen on the same port.
	//
	// This isn't a problem in pure OpenTelemetry Collector because it internally
	// deduplicates receiver instances, but since Flow don't have this logic we
	// need to reject these kinds of configs for now.
	if duplicateDiags := validateNoDuplicateReceivers(groups); len(duplicateDiags) > 0 {
		diags.AddAll(duplicateDiags)
		return diags
	}

	// TODO(rfratto): should this be deduplicated to avoid creating factories
	// twice?
	converterTable := buildConverterTable()

	for _, group := range groups {
		componentSets := []struct {
			kind         component.Kind
			ids          []component.ID
			configLookup map[component.ID]component.Config
		}{
			{component.KindReceiver, group.Receivers(), cfg.Receivers},
			{component.KindProcessor, group.Processors(), cfg.Processors},
			{component.KindExporter, group.Exporters(), cfg.Exporters},
		}

		for _, componentSet := range componentSets {
			for _, id := range componentSet.ids {
				componentID := component.InstanceID{Kind: componentSet.kind, ID: id}

				state := &state{
					cfg:   cfg,
					file:  file,
					group: &group,

					converterLookup: converterTable,

					componentConfig: componentSet.configLookup[id],
					componentID:     componentID,
				}

				key := converterKey{Kind: componentSet.kind, Type: id.Type()}
				conv, ok := converterTable[key]
				if !ok {
					panic(fmt.Sprintf("otelcolconvert: no converter found for key %v", key))
				}

				diags.AddAll(conv.ConvertAndAppend(state, componentID, componentSet.configLookup[id]))
			}
		}
	}

	return diags
}

// validateNoDuplicateReceivers validates that a given receiver does not appear
// in two different pipeline groups. This is required because Flow does not
// allow the same receiver to be instantiated more than once, while this is
// fine in OpenTelemetry due to internal deduplication rules.
func validateNoDuplicateReceivers(groups []pipelineGroup) diag.Diagnostics {
	var diags diag.Diagnostics

	usedReceivers := make(map[component.ID]struct{})

	for _, group := range groups {
		for _, receiver := range group.Receivers() {
			if _, found := usedReceivers[receiver]; found {
				diags.Add(diag.SeverityLevelCritical, fmt.Sprintf(
					"the configuration is unsupported because the receiver %q is used across multiple pipelines with distinct names",
					receiver.String(),
				))
			}
			usedReceivers[receiver] = struct{}{}
		}
	}

	return diags
}

func buildConverterTable() map[converterKey]componentConverter {
	table := make(map[converterKey]componentConverter)

	for _, conv := range converters {
		fact := conv.Factory()

		switch fact.(type) {
		case receiver.Factory:
			table[converterKey{Kind: component.KindReceiver, Type: fact.Type()}] = conv
		case processor.Factory:
			table[converterKey{Kind: component.KindProcessor, Type: fact.Type()}] = conv
		case exporter.Factory:
			table[converterKey{Kind: component.KindExporter, Type: fact.Type()}] = conv
		case connector.Factory:
			table[converterKey{Kind: component.KindConnector, Type: fact.Type()}] = conv
		case extension.Factory:
			table[converterKey{Kind: component.KindExtension, Type: fact.Type()}] = conv
		}
	}

	return table
}
