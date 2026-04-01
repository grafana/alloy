/**
 * layoutGraph contains the core Dagre-based graph layout logic shared between
 * the browser UI (ComponentGraph.tsx) and the standalone alloy-graph CLI tool.
 *
 * It is intentionally free of React / ReactFlow dependencies so it can be
 * consumed by Node.js scripts without a DOM.
 */
import dagre from '@dagrejs/dagre';

import { estimatedWidthOfNode } from './nodeUtils';

/** Minimum interface that a component must satisfy to be laid out. */
export interface GraphableComponent {
  localID: string;
  name: string;
  label?: string;
  moduleID: string;
  /** IDs of components this component sends data to (data-flow edges). */
  dataFlowEdgesTo: string[];
}

export interface LayoutNode {
  id: string;
  name: string;
  label?: string;
  moduleID: string;
  width: number;
  height: number;
  /** Centre x position assigned by Dagre. */
  x: number;
  /** Centre y position assigned by Dagre. */
  y: number;
}

export interface LayoutEdge {
  id: string;
  source: string;
  target: string;
  /** Index used to offset parallel edges between the same node pair. */
  edgeIndex: number;
}

export interface LayoutResult {
  nodes: LayoutNode[];
  edges: LayoutEdge[];
}

export const NODE_HEIGHT = 112;

/**
 * layoutGraph computes a left-to-right Dagre layout for the given components.
 *
 * Each component becomes a node; each entry in `dataFlowEdgesTo` becomes a
 * directed edge.  The returned positions are node *centres*.
 */
export function layoutGraph(components: GraphableComponent[]): LayoutResult {
  const graph = new dagre.graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}));
  graph.setGraph({ rankdir: 'LR' });

  const edgeCountMap: Record<string, number> = {};
  const edges: LayoutEdge[] = [];

  const nodes: LayoutNode[] = components.map((component) => {
    const width = estimatedWidthOfNode(component);

    const componentEdges: LayoutEdge[] = component.dataFlowEdgesTo.map((targetID) => {
      const edgeKey = `${component.localID}|${targetID}`;
      const count = edgeCountMap[edgeKey] ?? 0;
      edgeCountMap[edgeKey] = count + 1;
      return {
        id: `${edgeKey}|${count}`,
        source: component.localID,
        target: targetID,
        edgeIndex: count,
      };
    });
    edges.push(...componentEdges);

    return {
      id: component.localID,
      name: component.name,
      label: component.label,
      moduleID: component.moduleID,
      width,
      height: NODE_HEIGHT,
      x: 0,
      y: 0,
    };
  });

  nodes.forEach((node) => graph.setNode(node.id, { width: node.width, height: node.height }));
  edges.forEach((edge) => graph.setEdge(edge.source, edge.target, { label: edge.id }, edge.id));

  dagre.layout(graph);

  const layoutedNodes = nodes.map((node) => {
    const pos = graph.node(node.id);
    return { ...node, x: pos.x, y: pos.y };
  });

  return { nodes: layoutedNodes, edges };
}
