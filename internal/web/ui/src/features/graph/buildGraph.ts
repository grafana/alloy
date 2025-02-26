import dagre from '@dagrejs/dagre';
import { Edge, Node, Position } from '@xyflow/react';

import { ComponentInfo } from '../component/types';

import { DebugDataType } from './debugDataType';

const dagreGraph = new dagre.graphlib.Graph({ multigraph: true }).setDefaultEdgeLabel(() => ({}));

const nodeWidth = 150;
const nodeHeight = 72;
const position = { x: 0, y: 0 };

export function buildGraph(components: ComponentInfo[]): [Node[], Edge[]] {
  let edges: Edge[] = [];
  const nodes = components.map((component) => {
    let cpNameTruncated = component.name;
    if (cpNameTruncated.length > 25) {
      const parts = cpNameTruncated.split('.');
      parts!.shift();
      cpNameTruncated = parts.join('.');
    }
    const node: Node = {
      id: component.localID,
      width: nodeWidth,
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
      position: {
        x: nodeWithPosition.x - nodeWidth / 2,
        y: nodeWithPosition.y - nodeHeight / 2,
      },
    };
    return newNode;
  });

  return [newNodes, edges];
}

// The arrow direction of some components must be reversed in some cases.
function fixDirections(edges: Edge[]): Edge[] {
  return edges.map((edge) => {
    if (edge.target.startsWith('discovery.') || edge.target.startsWith('prometheus.exporter.')) {
      const tmp = edge.source;
      edge.source = edge.target;
      edge.target = tmp;
    }
    return edge;
  });
}
