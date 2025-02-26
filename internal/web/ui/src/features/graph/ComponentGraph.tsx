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

const ComponentGraph: React.FC<GraphProps> = ({ components, moduleID, enabled, window }) => {
  const navigate = useNavigate();
  const [layoutedNodes, layoutedEdges] = useMemo(() => buildGraph(components), [components]);
  const [data, setData] = useState<DebugData[]>([]);
  const { error } = useGraph(setData, moduleID, window, enabled, layoutedNodes);
  const [nodes, setNodes, onNodesChange] = useNodesState(layoutedNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layoutedEdges);

  const edgeTypes = {
    multiedge: MultiEdge,
  };

  useEffect(() => {
    setNodes(layoutedNodes);
    setEdges(layoutedEdges);
  }, [layoutedNodes, layoutedEdges]);

  // The API does not provide how many edges exist between two components.
  // This useEffect is using the debug data to add additional edges at runtime between components.
  useEffect(() => {
    // Sort by type to keep the edges in order when connected at the same node.
    const sortedDebugData = data.sort((a, b) => a.type.localeCompare(b.type));
    const newEdges: Edge[] = [];
    sortedDebugData.forEach((feed) => {
      const { localID } = parseID(feed.componentID);
      const targetComponentIDs = feed.targetComponentIDs
        ? feed.targetComponentIDs.map((targetID) => parseID(targetID).localID)
        : [];

      if (!targetComponentIDs || targetComponentIDs.length === 0) {
        const matches = edges.filter((edge) => edge.source === localID && edge.data!.signal == DebugDataType.UNDEFINED);
        matches.forEach((edge) => {
          edge.style = { stroke: DebugDataTypeColorMap[feed.type] };
          edge.label = feed.rate.toString();
          edge.data = { ...edge.data, signal: feed.type };
        });
      } else {
        targetComponentIDs.forEach((target) => {
          if (edges.find((edge) => edge.source === localID && edge.target === target && edge.data!.signal == feed.type)) {
            return; // already assigned
          }
          const matchUnassigned = edges.findIndex(
            (edge) => edge.source === localID && edge.target === target && edge.data!.signal == DebugDataType.UNDEFINED
          );
          if (matchUnassigned !== -1) {
            edges[matchUnassigned] = {
              ...edges[matchUnassigned],
              style: { stroke: DebugDataTypeColorMap[feed.type] },
              label: feed.rate.toString(),
              data: { ...edges[matchUnassigned].data, signal: feed.type },
            };
            return; // color an existing one
          }
          const matchAny = edges.filter((edge) => edge.source === localID && edge.target === target);
          if (matchAny && matchAny.length > 0) {
            newEdges.push({
              ...matchAny[0],
              id: matchAny[0].id + '|' + feed.type, // guarantees that it is unique
              style: { stroke: DebugDataTypeColorMap[feed.type] },
              label: feed.rate.toString(),
              data: { ...matchAny[0].data, signal: feed.type },
              // TODO: fix this weird hack to use the interactionWidth param here
              interactionWidth:
                matchAny.length + newEdges.filter((edge) => edge.source === localID && edge.target === target).length,
            });
          }
        });
      }
    });

    // Color the edges and add the label based on the debug data.
    setEdges((prevEdges) => {
      const updatedEdges = prevEdges.map((edge) => {
        const match = sortedDebugData.find(
          (item) => parseID(item.componentID).localID === edge.source && item.rate > 0 && edge.data!.signal === item.type
        );

        if (match) {
          return {
            ...edge,
            style: { stroke: DebugDataTypeColorMap[match.type] },
            label: match.rate.toString(),
            data: { ...edge.data },
          };
        }
        return {
          ...edge,
          style: { stroke: undefined },
          label: undefined,
          data: { ...edge.data },
        };
      });
      return [...updatedEdges, ...newEdges];
    });
  }, [data]);

  const onNodeClick = (event: any, node: Node) => {
    if (node.data.moduleID && node.data.moduleID != '') {
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

export default ComponentGraph;
