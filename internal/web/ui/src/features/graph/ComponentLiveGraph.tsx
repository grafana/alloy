import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
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
import { ComponentHealthState, ComponentInfo } from '../component/types';

import { buildGraph } from './buildGraph';
import { FeedData, FeedDataType, FeedDataTypeColorMap } from './feedDataType';
import { Legend } from './Legend';
import MultiEdge from './MultiEdge';

import '@xyflow/react/dist/style.css';

type LiveGraphProps = {
  components: ComponentInfo[];
};

const ComponentLiveGraph: React.FC<LiveGraphProps> = ({ components }) => {
  const [layoutedNodes, layoutedEdges] = useMemo(() => buildGraph(components), [components]);
  const [data, setData] = useState<FeedData[]>([]);
  const { error } = useLiveGraph(setData);

  const [nodes, _, onNodesChange] = useNodesState(layoutedNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layoutedEdges);

  const edgeTypes = {
    multiedge: MultiEdge,
  };

  // Ugly code to add some edges at runtime because we dont have this info from the Alloy graph
  useEffect(() => {
    const sortedFeedData = data.sort((a, b) => a.type.localeCompare(b.type));
    const newEdges: Edge[] = [];
    sortedFeedData.forEach((feed) => {
      const matchAny = edges.findIndex(
        (edge) =>
          edge.source === feed.componentID &&
          (!feed.targetComponentIDs ||
            feed.targetComponentIDs.length === 0 ||
            feed.targetComponentIDs.find((t) => t === edge.target))
      );
      const match = edges.findIndex(
        (edge) =>
          edge.source === feed.componentID &&
          edge.data!.signal == feed.type &&
          (!feed.targetComponentIDs ||
            feed.targetComponentIDs.length === 0 ||
            feed.targetComponentIDs.find((t) => t === edge.target))
      );
      const matchUnassigned = edges.findIndex(
        (edge) =>
          edge.source === feed.componentID &&
          edge.data!.signal == FeedDataType.UNDEFINED &&
          (!feed.targetComponentIDs ||
            feed.targetComponentIDs.length === 0 ||
            feed.targetComponentIDs.find((t) => t === edge.target))
      );
      if (match === -1 && matchUnassigned === -1 && matchAny !== -1) {
        newEdges.push({
          ...edges[matchAny],
          id: edges[matchAny].id + '|' + feed.type,
          style: { stroke: FeedDataTypeColorMap[feed.type] },
          label: feed.count.toString(),
          data: { ...edges[matchAny].data, signal: feed.type },
          interactionWidth:
            edges.filter(
              (edge) =>
                edge.source === feed.componentID &&
                (!feed.targetComponentIDs ||
                  feed.targetComponentIDs.length === 0 ||
                  feed.targetComponentIDs.find((t) => t === edge.target))
            ).length * 40,
        });
      } else if (match === -1 && matchUnassigned !== -1) {
        edges[matchUnassigned] = {
          ...edges[matchUnassigned],
          style: { stroke: FeedDataTypeColorMap[feed.type] },
          label: feed.count.toString(),
          data: { ...edges[matchUnassigned].data, signal: feed.type },
        };
      }
    });

    setEdges((prevEdges) => {
      const updatedEdges = prevEdges.map((edge) => {
        const match = sortedFeedData.find(
          (item) => item.componentID === edge.source && item.count > 0 && edge.data!.signal === item.type
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
          fitView
          attributionPosition="bottom-left"
          style={{ backgroundColor: '#F7F9FB' }}
        />
      )}
    </>
  );
};

export default ComponentLiveGraph;
