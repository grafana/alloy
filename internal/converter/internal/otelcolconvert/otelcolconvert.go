package otelcolconvert

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/syntax/token/builder"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/yamlprovider"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/receiver"
	"golang.org/x/exp/maps"
)

// This package is split into a set of [componentConverter] implementations
// which convert a single OpenTelemetry Collector component into one or more
// Alloy components.
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

	diags.AddAll(AppendConfig(f, cfg, "", nil))
	diags.AddAll(common.ValidateNodes(f))

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to render Alloy config: %s", err.Error()))
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
	configProvider, err := otelcol.NewConfigProvider(otelcol.ConfigProviderSettings{
		ResolverSettings: confmap.ResolverSettings{
			URIs:               []string{"yaml:" + string(in)},
			ProviderFactories:  []confmap.ProviderFactory{yamlprovider.NewFactory()},
			ConverterFactories: []confmap.ConverterFactory{expandconverter.NewFactory()},
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

// AppendConfig converts the provided OpenTelemetry config into an equivalent
// Alloy config and appends the result to the provided file.
func AppendConfig(file *builder.File, cfg *otelcol.Config, labelPrefix string, extraConverters []ComponentConverter) diag.Diagnostics {
	var diags diag.Diagnostics

	groups, err := createPipelineGroups(cfg.Service.Pipelines)
	if err != nil {
		diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("failed to interpret config: %s", err))
		return diags
	}
	// TODO(rfratto): should this be deduplicated to avoid creating factories
	// twice?
	converterTable := buildConverterTable(extraConverters)

	// Connector components are defined on the top level of the OpenTelemetry
	// config, but inside of the pipeline definitions they act like regular
	// receiver and exporter component IDs.
	// Connector components instances must _always_ be used both as an exporter
	// _and_ a receiver for the signal types they're supporting.
	//
	// Since we want to construct them individually, we'll exclude them from
	// the list of receivers and exporters manually.
	connectorIDs := maps.Keys(cfg.Connectors)

	// NOTE(rfratto): here, the same component ID will be instantiated once for
	// every group it's in. This means that converting receivers in multiple
	// groups will fail at runtime, as there will be two components attempting to
	// listen on the same port.
	//
	// This isn't a problem in pure OpenTelemetry Collector because it internally
	// deduplicates receiver instances, but since Alloy don't have this logic we
	// need to reject these kinds of configs for now.
	if duplicateDiags := validateNoDuplicateReceivers(groups, connectorIDs); len(duplicateDiags) > 0 {
		diags.AddAll(duplicateDiags)
		return diags
	}

	// We build the list of extensions 'activated' (defined in the service) as
	// Alloy components and keep a mapping of their OTel IDs to the blocks we've
	// built.
	// Since there's no concept of multiple extensions per group or telemetry
	// signal, we can build them before iterating over the groups.
	extensionTable := make(map[component.ID]componentID, len(cfg.Service.Extensions))

	for _, ext := range cfg.Service.Extensions {
		cidPtr := componentstatus.NewInstanceID(ext, component.KindExtension)
		cid := *cidPtr

		state := &State{
			cfg:  cfg,
			file: file,
			// We pass an empty pipelineGroup to make calls to
			// AlloyComponentLabel valid for both the converter authors and the
			// extension table mapping.
			group: &pipelineGroup{},

			converterLookup: converterTable,

			componentConfig:      cfg.Extensions,
			componentID:          cid,
			componentLabelPrefix: labelPrefix,
		}

		key := converterKey{Kind: component.KindExtension, Type: ext.Type()}
		conv, ok := converterTable[key]
		if !ok {
			panic(fmt.Sprintf("otelcolconvert: no converter found for key %v", key))
		}

		diags.AddAll(conv.ConvertAndAppend(state, cid, cfg.Extensions[ext]))

		extensionTable[ext] = componentID{
			Name:  strings.Split(conv.InputComponentName(), "."),
			Label: state.AlloyComponentLabel(),
		}
	}

	for _, group := range groups {
		receiverIDs := filterIDs(group.Receivers(), connectorIDs)
		processorIDs := group.Processors()
		exporterIDs := filterIDs(group.Exporters(), connectorIDs)

		componentSets := []struct {
			kind         component.Kind
			ids          []component.ID
			configLookup map[component.ID]component.Config
		}{
			{component.KindReceiver, receiverIDs, cfg.Receivers},
			{component.KindProcessor, processorIDs, cfg.Processors},
			{component.KindExporter, exporterIDs, cfg.Exporters},
			{component.KindConnector, connectorIDs, cfg.Connectors},
		}

		for _, componentSet := range componentSets {
			for _, id := range componentSet.ids {
				componentIDPtr := componentstatus.NewInstanceID(id, componentSet.kind)
				componentID := *componentIDPtr

				state := &State{
					cfg:   cfg,
					file:  file,
					group: &group,

					converterLookup: converterTable,
					extensionLookup: extensionTable,

					componentConfig:      componentSet.configLookup[id],
					componentID:          componentID,
					componentLabelPrefix: labelPrefix,
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
// in two different pipeline groups. This is required because Alloy does not
// allow the same receiver to be instantiated more than once, while this is
// fine in OpenTelemetry due to internal deduplication rules.
func validateNoDuplicateReceivers(groups []pipelineGroup, connectorIDs []component.ID) diag.Diagnostics {
	var diags diag.Diagnostics

	usedReceivers := make(map[component.ID]struct{})

	for _, group := range groups {
		receiverIDs := filterIDs(group.Receivers(), connectorIDs)
		for _, receiver := range receiverIDs {
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

func buildConverterTable(extraConverters []ComponentConverter) map[converterKey]ComponentConverter {
	table := make(map[converterKey]ComponentConverter)

	// Ordering is critical here because conflicting converters are resolved with
	// the first one in the list winning.
	allConverters := append(extraConverters, converters...)

	for _, conv := range allConverters {
		fact := conv.Factory()
		var kinds []component.Kind
		switch fact.(type) {
		case receiver.Factory:
			kinds = append(kinds, component.KindReceiver)
		case processor.Factory:
			kinds = append(kinds, component.KindProcessor)
		case exporter.Factory:
			kinds = append(kinds, component.KindExporter)
		case connector.Factory:
			kinds = append(kinds, component.KindConnector)
			// We need this so the connector is available as a destination for state.Next
			kinds = append(kinds, component.KindExporter)
			// Technically, this isn't required to be here since the entry
			// won't be required to look up a destination for state.Next, but
			// adding to reinforce the idea of how connectors are used.
			kinds = append(kinds, component.KindReceiver)
		case extension.Factory:
			kinds = append(kinds, component.KindExtension)
		}

		for _, kind := range kinds {
			// If a converter for this kind and type already exists, skip it.
			if _, ok := table[converterKey{Kind: kind, Type: fact.Type()}]; ok {
				continue
			}
			table[converterKey{Kind: kind, Type: fact.Type()}] = conv
		}
	}

	return table
}

func filterIDs(in []component.ID, rem []component.ID) []component.ID {
	var res []component.ID

	for _, set := range in {
		exists := false
		for _, id := range rem {
			if set == id {
				exists = true
			}
		}
		if !exists {
			res = append(res, set)
		}
	}

	return res
}
