import { type Edge, type Node, Position } from '@xyflow/react';

import type { ComponentInfo } from '../component/types';
import { DebugDataType } from './debugDataType';
import { layoutGraph, NODE_HEIGHT } from './layoutGraph';

/**
 * buildGraph converts a list of ComponentInfo objects into ReactFlow-compatible
 * Node and Edge arrays, using the shared layoutGraph function for positioning.
 */
export function buildGraph(components: ComponentInfo[]): [Node[], Edge[]] {
  const { nodes: layoutNodes, edges: layoutEdges } = layoutGraph(components);

  const nodes: Node[] = layoutNodes.map((n) => ({
    id: n.id,
    width: n.width,
    data: {
      label: n.name + ' "' + (n.label ?? '') + '"',
      localID: n.id,
      moduleID: n.moduleID,
    },
    // Dagre gives centre coordinates; ReactFlow expects the top-left corner.
    position: {
      x: n.x - n.width / 2,
      y: n.y - NODE_HEIGHT / 2,
    },
    targetPosition: Position.Left,
    sourcePosition: Position.Right,
  }));

  const edges: Edge[] = layoutEdges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    type: 'multiedge',
    animated: true,
    data: { signal: DebugDataType.UNDEFINED, edgeIndex: e.edgeIndex },
  }));

  return [nodes, edges];
}
