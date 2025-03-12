import dagre from '@dagrejs/dagre';
import { Edge, Node, Position } from '@xyflow/react';

import { ComponentInfo } from '../component/types';

import { DebugDataType } from './debugDataType';

const dagreGraph = new dagre.graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}));

// Arbitrary values chosen to fit an average config in the graph.
const NODE_WIDTH = 150;
const NODE_HEIGHT = 72;
const COMPONENT_NAME_MAX_LENGTH = 25;

const position = { x: 0, y: 0 };

export function buildGraph(components: ComponentInfo[]): [Node[], Edge[]] {
  let edges: Edge[] = [];
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
    const componentEdges: Edge[] = component.referencesTo.map((edge) => ({
      id: `${node.id}|${edge}`,
      source: node.id,
      target: edge,
      type: 'multiedge',
      animated: true,
      data: { signal: DebugDataType.UNDEFINED },
    }));
    edges.push(...componentEdges);
    return node;
  });

  edges = fixDirections(edges);

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

// The arrows of some components must be reversed because the graph that we receive from the backend
// is a dependency graph, not a data flow graph.
function fixDirections(edges: Edge[]): Edge[] {
  return edges.map((edge) => {
    if (edge.target.startsWith('discovery.') || edge.target.startsWith('prometheus.exporter.')) {
      const tmp = edge.source;
      edge.source = edge.target;
      edge.target = tmp;
      if (edge.id.includes('|')) {
        const parts = edge.id.split('|');
        edge.id = `${parts[1]}|${parts[0]}`;
      }
    }
    return edge;
  });
}
