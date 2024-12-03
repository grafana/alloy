import dagre from '@dagrejs/dagre';
import { Edge, Node, Position } from '@xyflow/react';

import { ComponentInfo } from '../component/types';

import { FeedDataType } from './feedDataType';

const dagreGraph = new dagre.graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}));

const nodeWidth = 172;
const nodeHeight = 36;
const position = { x: 0, y: 0 };

export function buildGraph(components: ComponentInfo[]): [Node[], Edge[]] {
  const edges: Edge[] = [];
  const nodes = components.map((component) => {
    const node: Node = {
      id: component.localID,
      data: {
        label: component.name.split('.').pop() + ' "' + (component.label ?? '') + '"',
        fullName: component.name + ' "' + (component.label ?? '') + '"',
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
