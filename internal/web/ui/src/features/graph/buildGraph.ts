import dagre from '@dagrejs/dagre';
import { Edge, Node, Position } from '@xyflow/react';

import { ComponentInfo } from '../component/types';

import { FeedDataType } from './feedDataType';

const dagreGraph = new dagre.graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}));

const nodeWidth = 200;
const nodeHeight = 36;
const position = { x: 0, y: 0 };

export function buildGraph(components: ComponentInfo[]): [Node[], Edge[]] {
  let edges: Edge[] = [];
  const nodes = components.map((component) => {
    const node: Node = {
      id: component.localID,
      width: nodeWidth,
      data: {
        label: component.name + ' "' + (component.label ?? '') + '"',
        localID: component.localID,
        moduleID: component.moduleID,
      },
      position: position,
    };
    const componentEdges: Edge[] = component.referencesTo.map((edge) => ({
      id: `${node.id}|${edge}`,
      source: node.id,
      target: edge,
      type: 'multiedge',
      animated: true,
      data: { signal: FeedDataType.UNDEFINED },
    }));
    edges.push(...componentEdges);
    return node;
  });

  edges = fixDirections(edges);

  dagreGraph.setGraph({ rankdir: 'LR' });

  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, { width: nodeWidth, height: nodeHeight });
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
      // We are shifting the dagre node position (anchor=center center) to the top left
      // so it matches the React Flow node anchor point (top left).
      position: {
        x: nodeWithPosition.x - nodeWidth / 2,
        y: nodeWithPosition.y - nodeHeight / 2,
      },
    };
    return newNode;
  });

  return [newNodes, edges];
}

function fixDirections(edges: Edge[]): Edge[] {
  return edges.map((edge) => {
    if (edge.target.startsWith('discovery.')) {
      const tmp = edge.source;
      edge.source = edge.target;
      edge.target = tmp;
    }
    return edge;
  });
}
