import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  addEdge,
  Background,
  ConnectionLineType,
  Edge,
  Node,
  Position,
  ReactFlow,
  useEdgesState,
  useNodesState,
} from '@xyflow/react';

import { useLiveGraph } from '../../hooks/liveGraph';
import { parseID } from '../../utils/id';
import { ComponentHealthState, ComponentInfo } from '../component/types';

import { buildGraph } from './buildGraph';
import { FeedData, FeedDataType, FeedDataTypeColorMap } from './feedDataType';
import { Legend } from './Legend';
import MultiEdge from './MultiEdge';

import '@xyflow/react/dist/style.css';

type LiveGraphProps = {
  components: ComponentInfo[];
  moduleID: string;
};

const ComponentLiveGraph: React.FC<LiveGraphProps> = ({ components, moduleID }) => {
  const navigate = useNavigate();
  const [layoutedNodes, layoutedEdges] = useMemo(() => buildGraph(components), [components]);
  const [data, setData] = useState<FeedData[]>([]);
  const [refreshSignal, setRefreshSignal] = useState(0);
  const { error } = useLiveGraph(setData, moduleID, refreshSignal);

  const [nodes, setNodes, onNodesChange] = useNodesState(layoutedNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layoutedEdges);

  const edgeTypes = {
    multiedge: MultiEdge,
  };

  useEffect(() => {
    setNodes(layoutedNodes);
    setEdges(layoutedEdges);
    setRefreshSignal((prev) => prev + 1);
  }, [layoutedNodes, layoutedEdges]);

  // Ugly code to add some edges at runtime because we dont have this info from the Alloy graph
  useEffect(() => {
    const sortedFeedData = data.sort((a, b) => a.type.localeCompare(b.type));
    const newEdges: Edge[] = [];
    sortedFeedData.forEach((feed) => {
      const { localID } = parseID(feed.componentID);
      const targetComponentIDs = feed.targetComponentIDs
        ? feed.targetComponentIDs.map((targetID) => parseID(targetID).localID)
        : [];
      if (!targetComponentIDs || targetComponentIDs.length === 0) {
        const matches = edges.filter((edge) => edge.source === localID && edge.data!.signal == FeedDataType.UNDEFINED);
        matches.forEach((edge) => {
          edge.style = { stroke: FeedDataTypeColorMap[feed.type] };
          edge.label = feed.count.toString();
          edge.data = { ...edge.data, signal: feed.type };
        });
      } else {
        targetComponentIDs.forEach((target) => {
          if (edges.find((edge) => edge.source === localID && edge.target === target && edge.data!.signal == feed.type)) {
            return; // already assigned
          }
          const matchUnassigned = edges.findIndex(
            (edge) => edge.source === localID && edge.target === target && edge.data!.signal == FeedDataType.UNDEFINED
          );
          if (matchUnassigned !== -1) {
            edges[matchUnassigned] = {
              ...edges[matchUnassigned],
              style: { stroke: FeedDataTypeColorMap[feed.type] },
              label: feed.count.toString(),
              data: { ...edges[matchUnassigned].data, signal: feed.type },
            };
            return; // color an existing one
          }
          const matchAny = edges.filter((edge) => edge.source === localID && edge.target === target);
          if (matchAny && matchAny.length > 0) {
            newEdges.push({
              ...matchAny[0],
              id: matchAny[0].id + '|' + feed.type, // bit ugly but that guarantees that it is unique
              style: { stroke: FeedDataTypeColorMap[feed.type] },
              label: feed.count.toString(),
              data: { ...matchAny[0].data, signal: feed.type },
              //weird hack to use the interactionWidth param here
              interactionWidth:
                matchAny.length + newEdges.filter((edge) => edge.source === localID && edge.target === target).length,
            });
          }
        });
      }
    });

    setEdges((prevEdges) => {
      const updatedEdges = prevEdges.map((edge) => {
        const match = sortedFeedData.find(
          (item) => parseID(item.componentID).localID === edge.source && item.count > 0 && edge.data!.signal === item.type
        );

        if (match) {
          return {
            ...edge,
            style: { stroke: FeedDataTypeColorMap[match.type] },
            label: match.count.toString(),
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

export default ComponentLiveGraph;
