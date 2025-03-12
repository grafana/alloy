import { useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Edge, Node, ReactFlow, useEdgesState, useNodesState } from '@xyflow/react';

import { useGraph } from '../../hooks/graph';
import { parseID } from '../../utils/id';
import { ComponentInfo } from '../component/types';

import { buildGraph } from './buildGraph';
import { DebugData, DebugDataType, DebugDataTypeColorMap } from './debugDataType';
import MultiEdge from './MultiEdge';

import '@xyflow/react/dist/style.css';

type GraphProps = {
  components: ComponentInfo[];
  moduleID: string;
  enabled: boolean;
  window: number;
};

// The graph is not updated on config reload. The page must be reloaded to see the changes.
const ComponentGraph: React.FC<GraphProps> = ({ components, moduleID, enabled, window }) => {
  const navigate = useNavigate();
  const [baseNodes, baseEdges] = useMemo(() => buildGraph(components), [components]);
  const [data, setData] = useState<DebugData[]>([]);
  const { error } = useGraph(setData, moduleID, window, enabled);

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const [nodes, setNodes, onNodesChange] = useNodesState(baseNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(baseEdges);

  const edgeTypes = {
    multiedge: MultiEdge,
  };

  // Some components, like the Otel ones, can send different types of data to the same component.
  // This is not an information that we can get from the dependency graph, so we need to adjust it at runtime by
  // checking the edges that we have and the data that we receive to add the missing edges.
  useEffect(() => {
    // Sort by type to keep the edges in order when connected at the same node.
    const sortedDebugData = [...data].sort((a, b) => a.type.localeCompare(b.type));

    setEdges((prevEdges) => {
      const workingEdges = [...prevEdges];
      const newEdges: Edge[] = [];

      sortedDebugData.forEach((debugData) => {
        const { localID } = parseID(debugData.componentID);
        const targetComponentIDs = debugData.targetComponentIDs
          ? debugData.targetComponentIDs.map((targetID) => parseID(targetID).localID)
          : [];

        if (targetComponentIDs.length === 0) {
          processDebugDataWithoutTargets(workingEdges, localID, debugData);
        } else {
          targetComponentIDs.forEach((target) => {
            processDebugDataWithTargets(workingEdges, newEdges, localID, target, debugData);
          });
        }
      });

      // Reset styles for edges did not receive data during the current window.
      resetUnmatchedEdges(workingEdges, sortedDebugData);

      return [...workingEdges, ...newEdges];
    });
  }, [setEdges, data]);

  // On click, open the component details page.
  const onNodeClick = (_event: React.MouseEvent, node: Node) => {
    if (node.data.moduleID && node.data.moduleID !== '') {
      navigate(`/component/${node.data.moduleID}/${node.data.localID}`);
    } else {
      navigate(`/component/${node.data.localID}`);
    }
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

// The debug data coming from the component only corresponds to specific edges.
// If the initial edge is already used by another signal type, we need to create a new edge with the new signal type.
function processDebugDataWithTargets(
  edges: Edge[],
  newEdges: Edge[],
  sourceID: string,
  targetID: string,
  debugData: DebugData
) {
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

  // If there is no edge that is not assigned to any signal type, we need to create a new edge.
  const matchingEdges = edges.filter((edge) => edge.source === sourceID && edge.target === targetID);

  if (matchingEdges.length > 0) {
    const existingEdgesCount = matchingEdges.length;
    const newEdgesCount = newEdges.filter((edge) => edge.source === sourceID && edge.target === targetID).length;

    newEdges.push({
      ...matchingEdges[0],
      id: matchingEdges[0].id + '|' + debugData.type, // Guarantees uniqueness
      style: { stroke: DebugDataTypeColorMap[debugData.type] },
      label: debugData.rate.toLocaleString(undefined, { maximumFractionDigits: 2, minimumFractionDigits: 0 }),
      data: { ...(matchingEdges[0].data || {}), signal: debugData.type, edgeIndex: existingEdgesCount + newEdgesCount },
    });
  }
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
