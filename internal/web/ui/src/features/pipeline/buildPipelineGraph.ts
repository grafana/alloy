import {
  deduplicateSignals,
  hasInferredSignals,
  inferPipelineStage,
  inferSignalsFromComponentName,
  type PipelineEdge,
  type PipelineGraphData,
  type PipelineNode,
  type PipelineNodeHealth,
  SignalKind,
} from '@grafana/alloy-pipeline-graph';

import { componentDocsUrl } from '../../utils/docs';
import type { ComponentHealthState, ComponentInfo } from '../component/types';

const INNER_NODE_ID_SEPARATOR = '/';

function innerNodeId(containerId: string, innerLocalId: string): string {
  return `${containerId}${INNER_NODE_ID_SEPARATOR}${innerLocalId}`;
}

function mapHealth(state: ComponentHealthState): PipelineNodeHealth {
  return state as PipelineNodeHealth;
}

function buildComponentNode(component: ComponentInfo, overrides: Partial<PipelineNode> = {}): PipelineNode {
  return {
    id: component.localID,
    componentName: component.name,
    label: component.label ?? null,
    stage: inferPipelineStage(component.name),
    signals: deduplicateSignals(inferSignalsFromComponentName(component.name)),
    docsUrl: componentDocsUrl(component.name),
    health: mapHealth(component.health.state),
    meta: {
      moduleID: component.moduleID,
      localID: component.localID,
    },
    ...overrides,
  };
}

function inferDataFlowSignals(sourceName: string, targetName: string | undefined): SignalKind[] {
  const sourceSignals = inferSignalsFromComponentName(sourceName);
  if (hasInferredSignals(sourceSignals)) {
    return deduplicateSignals(sourceSignals);
  }
  if (targetName) {
    return deduplicateSignals(inferSignalsFromComponentName(targetName));
  }
  return [SignalKind.Other];
}

function addEdges(
  edges: PipelineEdge[],
  edgeIdSet: Set<string>,
  components: ComponentInfo[],
  idForComponent: (localID: string) => string,
  nameByLocalID: Map<string, string>
): void {
  for (const component of components) {
    const sourceID = idForComponent(component.localID);
    for (const targetLocalID of component.dataFlowEdgesTo) {
      const targetID = idForComponent(targetLocalID);
      const signals = inferDataFlowSignals(component.name, nameByLocalID.get(targetLocalID));

      let edgeId = `${sourceID}->${targetID}`;
      let counter = 0;
      while (edgeIdSet.has(edgeId)) {
        counter += 1;
        edgeId = `${sourceID}->${targetID}#${counter}`;
      }
      edgeIdSet.add(edgeId);

      edges.push({ id: edgeId, source: sourceID, target: targetID, signals });
    }
  }
}

/**
 * Builds the shared `PipelineGraphData` from Alloy's runtime component API.
 *
 * Custom components (module loaders) become expandable containers when
 * `moduleInternals` includes their `createdModuleIDs` entries.
 */
export function buildPipelineGraph(
  components: ComponentInfo[],
  moduleInternals: Map<string, ComponentInfo[]> = new Map()
): PipelineGraphData {
  const nameByLocalID = new Map<string, string>();
  for (const component of components) {
    nameByLocalID.set(component.localID, component.name);
  }

  const nodes: PipelineNode[] = [];
  const edges: PipelineEdge[] = [];
  const edgeIdSet = new Set<string>();

  for (const component of components) {
    const isContainer = (component.createdModuleIDs?.length ?? 0) > 0;

    nodes.push(
      buildComponentNode(component, {
        kind: isContainer ? 'customComponentContainer' : 'component',
      })
    );

    if (!isContainer) {
      continue;
    }

    const innerComponents: ComponentInfo[] = [];
    for (const moduleId of component.createdModuleIDs ?? []) {
      const moduleComponents = moduleInternals.get(moduleId) ?? [];
      innerComponents.push(...moduleComponents);
      for (const inner of moduleComponents) {
        nameByLocalID.set(inner.localID, inner.name);
      }
    }

    for (const inner of innerComponents) {
      nodes.push(
        buildComponentNode(inner, {
          id: innerNodeId(component.localID, inner.localID),
          parentId: component.localID,
          kind: 'component',
          meta: {
            moduleID: inner.moduleID,
            localID: inner.localID,
            containerID: component.localID,
          },
        })
      );
    }

    addEdges(edges, edgeIdSet, innerComponents, (localID) => innerNodeId(component.localID, localID), nameByLocalID);
  }

  addEdges(edges, edgeIdSet, components, (localID) => localID, nameByLocalID);

  return { nodes, edges };
}
