import { useEffect, useMemo, useState } from 'react';
import { type Edge, type Node, ReactFlow, useEdgesState, useNodesState } from '@xyflow/react';

import { useGraph } from '../../hooks/graph';
import { parseID } from '../../utils/id';
import type { ComponentInfo } from '../component/types';

import { buildGraph } from './buildGraph';
import { type DebugData, DebugDataType, DebugDataTypeColorMap } from './debugDataType';
import MultiEdge from './MultiEdge';

import '@xyflow/react/dist/style.css';
import { usePathPrefix } from '../../contexts/usePathPrefix';

type GraphProps = {
  components: ComponentInfo[];
  moduleID: string;
  enabled: boolean;
  window: number;
};

// The graph is not updated on config reload. The page must be reloaded to see the changes.
const ComponentGraph: React.FC<GraphProps> = ({ components, moduleID, enabled, window }) => {
  const pathPrefix = usePathPrefix();
  const [baseNodes, baseEdges] = useMemo(() => buildGraph(components), [components]);
  const [data, setData] = useState<DebugData[]>([]);
  const { error } = useGraph(setData, moduleID, window, enabled);

  const [nodes, _, onNodesChange] = useNodesState(baseNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(baseEdges);

  const edgeTypes = {
    multiedge: MultiEdge,
  };

  // Update the edges with the new data.
  useEffect(() => {
    // Sort by type to keep the edges in order when connected at the same node.
    const sortedDebugData = [...data].sort((a, b) => a.type.localeCompare(b.type));

    setEdges((prevEdges) => {
      const workingEdges = [...prevEdges];

      sortedDebugData.forEach((debugData) => {
        const { localID } = parseID(debugData.componentID);
        const targetComponentIDs = debugData.targetComponentIDs
          ? debugData.targetComponentIDs.map((targetID) => parseID(targetID).localID)
          : [];

        if (targetComponentIDs.length === 0) {
          processDebugDataWithoutTargets(workingEdges, localID, debugData);
        } else {
          targetComponentIDs.forEach((target) => {
            processDebugDataWithTargets(workingEdges, localID, target, debugData);
          });
        }
      });

      // Reset styles for edges did not receive data during the current window.
      resetUnmatchedEdges(workingEdges, sortedDebugData);

      return workingEdges;
    });
  }, [setEdges, data]);

  // On click, open the component details page in a new tab.
  const onNodeClick = (_event: React.MouseEvent, node: Node) => {
    const baseUrl = globalThis.window.location.origin + pathPrefix;
    const moduleID = typeof node.data.moduleID === 'string' ? node.data.moduleID : '';
    const remoteCfgPrefix = moduleID.startsWith('remotecfg/') ? 'remotecfg/' : '';
    const path =
      node.data.moduleID && node.data.moduleID !== ''
        ? `component/${node.data.moduleID}/${node.data.localID}`
        : `component/${node.data.localID}`;

    globalThis.window.open(baseUrl + remoteCfgPrefix + path, '_blank');
  };

  return (
    <>
      {error ? (
        <p>Error: {error}</p>
      ) : (
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          edgeTypes={edgeTypes}
          onNodeClick={onNodeClick}
          fitView
          attributionPosition="bottom-left"
          style={{ backgroundColor: '#F7F9FB' }}
          minZoom={0.1}
        />
      )}
    </>
  );
};

// The debug data does not have any specific target ids. We can color all existing edges that match the source id.
function processDebugDataWithoutTargets(edges: Edge[], sourceID: string, debugData: DebugData) {
  const matches = edges.filter((edge) => edge.source === sourceID && edge.data?.signal === DebugDataType.UNDEFINED);

  matches.forEach((edge) => {
    edge.style = { stroke: DebugDataTypeColorMap[debugData.type] };
    edge.label = debugData.rate.toLocaleString(undefined, { maximumFractionDigits: 2, minimumFractionDigits: 0 });
    edge.data = { ...(edge.data || {}), signal: debugData.type };
  });
}

// The debug data coming from the component corresponds only to specific edges.
// We need to find the right edge and update it with the new data.
function processDebugDataWithTargets(edges: Edge[], sourceID: string, targetID: string, debugData: DebugData) {
  const existingEdge = edges.find(
    (edge) => edge.source === sourceID && edge.target === targetID && edge.data?.signal === debugData.type
  );

  if (existingEdge) {
    return;
  }

  const unassignedEdgeIndex = edges.findIndex(
    (edge) => edge.source === sourceID && edge.target === targetID && edge.data?.signal === DebugDataType.UNDEFINED
  );

  // If there is an edge that is not assigned to any signal type, we can use it.
  if (unassignedEdgeIndex !== -1) {
    const edgeData = edges[unassignedEdgeIndex].data || {};
    edges[unassignedEdgeIndex] = {
      ...edges[unassignedEdgeIndex],
      style: { stroke: DebugDataTypeColorMap[debugData.type] },
      label: debugData.rate.toLocaleString(undefined, { maximumFractionDigits: 2, minimumFractionDigits: 0 }),
      data: { ...edgeData, signal: debugData.type },
    };
    return;
  }

  // The backend should provide enough edges to match the data. If this is triggered, then it
  // means that the data is not aligned with the edges.
  console.warn('No edge found, data will not be displayed', sourceID, targetID, debugData);
}

// Reset styles for edges did not receive data during the current window.
function resetUnmatchedEdges(edges: Edge[], debugData: DebugData[]) {
  edges.forEach((edge) => {
    const match = debugData.find(
      (item) => parseID(item.componentID).localID === edge.source && item.rate > 0 && edge.data?.signal === item.type
    );

    if (match) {
      edge.style = { stroke: DebugDataTypeColorMap[match.type] };
      edge.label = match.rate.toLocaleString(undefined, { maximumFractionDigits: 2, minimumFractionDigits: 0 });
    } else {
      edge.style = { stroke: undefined };
      edge.label = undefined;
    }
  });
}

export default ComponentGraph;
