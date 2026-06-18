import type { PipelineGraphData } from '@grafana/alloy-pipeline-graph';

import { parseID } from '../../utils/id';
import type { DebugData } from './debugDataType';

/**
 * Overlays live debug stream rates onto pipeline graph edges as optional `metric`
 * fields. When no debug data matches an edge, any existing metric is cleared.
 */
export function overlayLiveMetrics(graph: PipelineGraphData, debugData: DebugData[]): PipelineGraphData {
  const sortedDebugData = [...debugData].sort((a, b) => a.type.localeCompare(b.type));
  const edges = graph.edges.map((edge) => ({ ...edge }));

  for (const item of sortedDebugData) {
    const sourceLocalID = parseID(item.componentID).localID;
    const targetLocalIDs = item.targetComponentIDs?.map((targetID) => parseID(targetID).localID) ?? [];

    if (targetLocalIDs.length === 0) {
      assignMetric(edges, (edge) => edge.source === sourceLocalID && edge.metric === undefined, item.rate);
      continue;
    }

    for (const targetLocalID of targetLocalIDs) {
      assignMetric(
        edges,
        (edge) => edge.source === sourceLocalID && edge.target === targetLocalID && edge.metric === undefined,
        item.rate
      );
    }
  }

  for (const edge of edges) {
    const sourceLocalID = edge.source.includes('/') ? edge.source.split('/').pop()! : edge.source;
    const targetLocalID = edge.target.includes('/') ? edge.target.split('/').pop()! : edge.target;

    const match = sortedDebugData.find((item) => {
      const itemSource = parseID(item.componentID).localID;
      if (itemSource !== sourceLocalID && itemSource !== edge.source) {
        return false;
      }
      if (item.rate <= 0) {
        return false;
      }
      if (!item.targetComponentIDs || item.targetComponentIDs.length === 0) {
        return itemSource === edge.source || itemSource === sourceLocalID;
      }
      return item.targetComponentIDs.some((targetID) => {
        const parsed = parseID(targetID).localID;
        return parsed === targetLocalID || parsed === edge.target;
      });
    });

    if (match) {
      edge.metric = { value: match.rate };
    } else {
      delete edge.metric;
    }
  }

  return { ...graph, edges };
}

function assignMetric(
  edges: PipelineGraphData['edges'],
  predicate: (edge: PipelineGraphData['edges'][number]) => boolean,
  rate: number
): void {
  const edge = edges.find(predicate);
  if (edge) {
    edge.metric = { value: rate };
  }
}
