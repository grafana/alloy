import { type PipelineEdgeSignalMetric, type PipelineGraphData, SignalKind } from '@grafana/alloy-pipeline-graph';

import { parseID } from '../../utils/id';
import { type DebugData, DebugDataType } from './debugDataType';

/**
 * Maps a live debug-stream data type onto the signal a graph edge carries.
 * `target` (service-discovery counts) gets its own signal so it stays visually
 * distinct from telemetry; `undefined` is the frontend's placeholder and never
 * arrives from the backend.
 */
function debugTypeToSignal(type: DebugDataType): SignalKind {
  switch (type) {
    case DebugDataType.PROMETHEUS_METRIC:
    case DebugDataType.OTEL_METRIC:
      return SignalKind.Metrics;
    case DebugDataType.LOKI_LOG:
    case DebugDataType.OTEL_LOG:
      return SignalKind.Logs;
    case DebugDataType.OTEL_TRACE:
      return SignalKind.Traces;
    case DebugDataType.TARGET:
      return SignalKind.Targets;
    default:
      return SignalKind.Other;
  }
}

/**
 * Resolves an edge endpoint to a component localID. Inner (custom-component)
 * nodes carry a `containerID/localID` id, so strip the container prefix to
 * compare against the debug stream's component IDs.
 */
function endpointLocalID(endpoint: string): string {
  return endpoint.includes('/') ? endpoint.split('/').pop()! : endpoint;
}

/**
 * Overlays the live `/graph` debug stream onto pipeline graph edges as
 * per-signal flow rates (`signalMetrics`). Each debug item reports a rate of a
 * given signal type flowing from a component to its targets; rates are grouped
 * per edge and per signal so a single edge can show metrics, logs and traces at
 * independent rates. When no debug data matches an edge in the current window,
 * any stale metric is cleared.
 */
export function overlayLiveMetrics(graph: PipelineGraphData, debugData: DebugData[]): PipelineGraphData {
  const edges = graph.edges.map((edge) => {
    const sourceLocalID = endpointLocalID(edge.source);
    const targetLocalID = endpointLocalID(edge.target);

    const ratesBySignal = new Map<SignalKind, number>();
    for (const item of debugData) {
      if (item.rate <= 0) {
        continue;
      }

      const itemSource = parseID(item.componentID).localID;
      if (itemSource !== sourceLocalID && itemSource !== edge.source) {
        continue;
      }

      const hasTargets = (item.targetComponentIDs?.length ?? 0) > 0;
      const matchesTarget =
        !hasTargets ||
        item.targetComponentIDs.some((targetID) => {
          const parsed = parseID(targetID).localID;
          return parsed === targetLocalID || parsed === edge.target;
        });
      if (!matchesTarget) {
        continue;
      }

      const signal = debugTypeToSignal(item.type);
      ratesBySignal.set(signal, (ratesBySignal.get(signal) ?? 0) + item.rate);
    }

    if (ratesBySignal.size === 0) {
      // No live data for this edge in the current window: drop any stale metric.
      const { metric: _metric, signalMetrics: _signalMetrics, ...rest } = edge;
      return rest;
    }

    const signalMetrics: PipelineEdgeSignalMetric[] = Array.from(ratesBySignal.entries()).map(([signal, value]) => ({
      signal,
      value,
    }));

    return { ...edge, signalMetrics };
  });

  return { ...graph, edges };
}
