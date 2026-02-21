package otelcolconvert

import (
	"cmp"
	"fmt"
	slices0 "slices"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/service/pipelines"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

// pipelineGroup groups a set of pipelines together by their telemetry type.
type pipelineGroup struct {
	// Name of the group. May be an empty string.
	Name string

	Metrics *pipelines.PipelineConfig
	Logs    *pipelines.PipelineConfig
	Traces  *pipelines.PipelineConfig
}

// createPipelineGroups groups pipelines of different telemetry types together
// by the user-specified pipeline name. For example, the following
// configuration creates two groups:
//
//	# (component definitions are omitted for brevity)
//
//	pipelines:
//	  metrics: # ID: metrics/<empty>
//	    receivers: [otlp]
//	    exporters: [otlp]
//	  logs: # ID: logs/<empty
//	    receivers: [otlp]
//	    exporters: [otlp]
//	  metrics/2: # ID: metrics/2
//	    receivers: [otlp/2]
//	    exporters: [otlp/2]
//	  traces/2: # ID: traces/2
//	    receivers: [otlp/2]
//	    exporters: [otlp/2]
//
// Here, the two groups are [metrics/<empty> logs/<empty>] and [metrics/2
// traces/2]. The key used for grouping is the name of the pipeline, so that
// pipelines with matching names belong to the same group.
//
// This allows us to emit an Alloy-native pipeline, where one component is
// responsible for multiple telemetry types, as opposed as to creating the
// otlp/2 receiver two separate times (once for metrics and once for traces).
//
// Note that OpenTelemetry guaratees that the pipeline name is unique, so there
// can't be two pipelines called metrics/2; any given pipeline group is
// guaranteed to contain at most one pipeline of each telemetry type.
func createPipelineGroups(cfg pipelines.Config) ([]pipelineGroup, error) {
	groups := map[string]pipelineGroup{}

	for key, config := range cfg {
		name := key.Name()
		group := groups[name]
		group.Name = name

		switch key.Signal() {
		case pipeline.SignalMetrics:
			if group.Metrics != nil {
				return nil, fmt.Errorf("duplicate metrics pipeline for pipeline named %q", name)
			}
			group.Metrics = config
		case pipeline.SignalLogs:
			if group.Logs != nil {
				return nil, fmt.Errorf("duplicate logs pipeline for pipeline named %q", name)
			}
			group.Logs = config
		case pipeline.SignalTraces:
			if group.Traces != nil {
				return nil, fmt.Errorf("duplicate traces pipeline for pipeline named %q", name)
			}
			group.Traces = config
		default:
			return nil, fmt.Errorf("unknown pipeline type %q", key.Signal())
		}

		groups[name] = group
	}

	// Initialize created groups.
	for key, group := range groups {
		if group.Metrics == nil {
			group.Metrics = &pipelines.PipelineConfig{}
		}
		if group.Logs == nil {
			group.Logs = &pipelines.PipelineConfig{}
		}
		if group.Traces == nil {
			group.Traces = &pipelines.PipelineConfig{}
		}
		groups[key] = group
	}

	res := maps.Values(groups)
	slices.SortStableFunc(res, func(a, b pipelineGroup) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return res, nil
}

// Receivers returns a set of unique IDs for receivers across all telemetry
// types.
func (group pipelineGroup) Receivers() []component.ID {
	return mergeIDs(
		group.Metrics.Receivers,
		group.Logs.Receivers,
		group.Traces.Receivers,
	)
}

// Processors returns a set of unique IDs for processors across all telemetry
// types.
func (group pipelineGroup) Processors() []component.ID {
	return mergeIDs(
		group.Metrics.Processors,
		group.Logs.Processors,
		group.Traces.Processors,
	)
}

// Exporters returns a set of unique IDs for exporters across all telemetry
// types.
func (group pipelineGroup) Exporters() []component.ID {
	return mergeIDs(
		group.Metrics.Exporters,
		group.Logs.Exporters,
		group.Traces.Exporters,
	)
}

// mergeIDs merges a set of IDs into a unique list.
func mergeIDs(in ...[]component.ID) []component.ID {
	var res []component.ID

	unique := map[component.ID]struct{}{}

	for _, set := range in {
		for _, id := range set {
			if _, exists := unique[id]; exists {
				continue
			}

			res = append(res, id)
			unique[id] = struct{}{}
		}
	}

	return res
}

// NextMetrics returns the set of components who should be sent metrics from
// the given component ID.
func (group pipelineGroup) NextMetrics(fromID componentstatus.InstanceID) []componentstatus.InstanceID {
	return nextInPipeline(group.Metrics, fromID)
}

// NextLogs returns the set of components who should be sent logs from the
// given component ID.
func (group pipelineGroup) NextLogs(fromID componentstatus.InstanceID) []componentstatus.InstanceID {
	return nextInPipeline(group.Logs, fromID)
}

// NextTraces returns the set of components who should be sent traces from the
// given component ID.
func (group pipelineGroup) NextTraces(fromID componentstatus.InstanceID) []componentstatus.InstanceID {
	return nextInPipeline(group.Traces, fromID)
}

func nextInPipeline(pipeline *pipelines.PipelineConfig, fromID componentstatus.InstanceID) []componentstatus.InstanceID {
	switch fromID.Kind() {
	case component.KindReceiver, component.KindConnector:
		// Validate this receiver is part of the pipeline.
		if !findInComponentIds(fromID, pipeline.Receivers) {
			return nil
		}

		// Receivers and connectors should either send to the first processor
		// if one exists or to every exporter otherwise.
		if len(pipeline.Processors) > 0 {
			return []componentstatus.InstanceID{*componentstatus.NewInstanceID(pipeline.Processors[0], component.KindProcessor)}
		}
		return toComponentInstanceIDs(component.KindExporter, pipeline.Exporters)

	case component.KindProcessor:
		// Validate this processor is part of the pipeline.
		if !findInComponentIds(fromID, pipeline.Processors) {
			return nil
		}

		// Processors should send to the next processor if one exists or to every
		// exporter otherwise.
		processorIndex := slices.Index(pipeline.Processors, fromID.ComponentID())
		if processorIndex+1 < len(pipeline.Processors) {
			// Send to next processor.
			return []componentstatus.InstanceID{*componentstatus.NewInstanceID(pipeline.Processors[processorIndex+1], component.KindProcessor)}
		}

		return toComponentInstanceIDs(component.KindExporter, pipeline.Exporters)

	case component.KindExporter:
		// Exporters never send to another otelcol component.
		return nil

	default:
		panic(fmt.Sprintf("nextInPipeline: unsupported component kind %v", fromID.Kind()))
	}
}

// toComponentInstanceIDs converts a slice of [component.ID] into a slice of
// [componentstatus.InstanceID]. Each element in the returned slice will have a
// kind matching the provided kind argument.
func toComponentInstanceIDs(kind component.Kind, ids []component.ID) []componentstatus.InstanceID {
	res := make([]componentstatus.InstanceID, 0, len(ids))

	for _, id := range ids {
		res = append(res, *componentstatus.NewInstanceID(id, kind))
	}

	return res
}

func findInComponentIds(fromID componentstatus.InstanceID, componentIDs []component.ID) bool {
	return slices0.Contains(componentIDs, fromID.ComponentID())
}
