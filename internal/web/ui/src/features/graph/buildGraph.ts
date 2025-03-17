import dagre from '@dagrejs/dagre';
import { Edge, Node, Position } from '@xyflow/react';

import { ComponentInfo } from '../component/types';

import { DebugDataType } from './debugDataType';

const dagreGraph = new dagre.graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}));

// Arbitrary values chosen to fit an average config in the graph.
const NODE_WIDTH = 155;
const NODE_HEIGHT = 72;
const COMPONENT_NAME_MAX_LENGTH = 25;

const position = { x: 0, y: 0 };

export function buildGraph(components: ComponentInfo[]): [Node[], Edge[]] {
  const edges: Edge[] = [];
  // Map to track the count of edges between the same source and target
  const edgeCountMap: Record<string, number> = {};

  const nodes = components.map((component) => {
    let cpNameTruncated = component.name;
    if (cpNameTruncated.length > COMPONENT_NAME_MAX_LENGTH) {
      const parts = cpNameTruncated.split('.');
      parts.shift();
      cpNameTruncated = parts.join('.');
    }
    const node: Node = {
      id: component.localID,
      width: NODE_WIDTH,
      data: {
        label: cpNameTruncated + ' "' + (component.label ?? '') + '"',
        localID: component.localID,
        moduleID: component.moduleID,
      },
      position: position,
    };
    const componentEdges: Edge[] = component.dataFlowEdgesTo.map((edge) => {
      const edgeKey = `${node.id}|${edge}`;
      const count = edgeCountMap[edgeKey] || 0;
      edgeCountMap[edgeKey] = count + 1;

      return {
        id: `${edgeKey}|${count}`,
        source: node.id,
        target: edge,
        type: 'multiedge',
        animated: true,
        data: { signal: DebugDataType.UNDEFINED, edgeIndex: count },
      };
    });
    edges.push(...componentEdges);
    return node;
  });

  dagreGraph.setGraph({ rankdir: 'LR' });

  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  });

  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target, { label: edge.id }, edge.id);
  });

  dagre.layout(dagreGraph);

  const newNodes: Node[] = nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    const newNode: Node = {
      ...node,
      targetPosition: Position.Left,
      sourcePosition: Position.Right,
      position: {
        x: nodeWithPosition.x - NODE_WIDTH / 2,
        y: nodeWithPosition.y - NODE_HEIGHT / 2,
      },
    };
    return newNode;
  });

  return [newNodes, edges];
}
