import { useCallback, useEffect, useRef, useState } from 'react';
import { addEdge, Background, Node, Position, ReactFlow, useEdgesState, useNodesState } from '@xyflow/react';

import { useLiveGraph } from '../../hooks/liveGraph';
import { ComponentHealthState, ComponentInfo } from '../component/types';

import { buildGraph } from './buildGraph';
import { FeedData } from './feedDataType';

import '@xyflow/react/dist/style.css';

type LiveGraphProps = {
  components: ComponentInfo[];
};

const ComponentLiveGraph: React.FC<LiveGraphProps> = ({ components }) => {
  const [layoutedNodes, layoutedEdges] = buildGraph(components);
  const [data, setData] = useState<FeedData[]>([]);
  const { error } = useLiveGraph(setData);

  const [nodes, _, onNodesChange] = useNodesState(layoutedNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layoutedEdges);

  const dataRef = useRef(data); // Reference to the latest data
  const updateEdges = useCallback(() => {
    setEdges((prevEdges) =>
      prevEdges.map((edge) => {
        const matchingDataIndex = dataRef.current.findIndex((item) => item.componentID === edge.source && item.count > 0);

        if (matchingDataIndex !== -1) {
          const matchingData = dataRef.current[matchingDataIndex];

          // Update data immutably
          const newData = [...dataRef.current];
          newData.splice(matchingDataIndex, 1);
          dataRef.current = newData;
          setData(newData);

          return {
            ...edge,
            style: { stroke: 'red' },
            label: matchingData.count.toString(),
            data: { ...edge.data },
          };
        }

        // Reset edges that don’t match
        return {
          ...edge,
          style: { stroke: undefined },
          label: undefined,
          data: { ...edge.data },
        };
      })
    );
  }, [setEdges]);

  useEffect(() => {
    dataRef.current = data; // Keep ref updated with the latest data
  }, [data]);

  useEffect(() => {
    const interval = setInterval(updateEdges, 2000); // Run update every 2 seconds

    return () => clearInterval(interval); // Cleanup interval on unmount
  }, [updateEdges]); // `updateEdges` is memoized

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      fitView
      attributionPosition="bottom-left"
      style={{ backgroundColor: '#F7F9FB' }}
    >
      <Background />
    </ReactFlow>
  );
};

export default ComponentLiveGraph;
