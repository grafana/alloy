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

  useEffect(() => {
    setEdges((prevEdges) =>
      prevEdges.map((edge) => {
        const match = data.find((item) => item.componentID === edge.source && item.count > 0);

        if (match) {
          return {
            ...edge,
            style: { stroke: 'red' },
            label: match.count.toString(),
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
  }, [data]);

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
