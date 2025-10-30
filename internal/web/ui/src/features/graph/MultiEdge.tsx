import { BaseEdge, type EdgeProps, EdgeText, getBezierPath, getStraightPath } from '@xyflow/react';
import { getSpecialPath } from './getSpecialPath';

export type GetSpecialPathParams = {
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
};

// This is a custom edge because by default the xyflow library does not support multi-edges between two nodes.
export default function CustomEdge({
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style,
  label,
  data,
}: EdgeProps) {
  const edgePathParams = {
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  };

  let path = '';

  const edgeIndex = Number(data?.edgeIndex) || 0;
  const offset = 50 * edgeIndex * (edgeIndex % 2 === 0 ? 1 : -1);

  // Detect if the edge is mostly horizontal
  const isHorizontal = Math.abs(targetY - sourceY) < 10;

  // Horizontal edges with no offset should use a straight path because the bezier path makes the animation look weird.
  if (isHorizontal && offset === 0) {
    [path] = getStraightPath(edgePathParams);
  } else if (offset !== 0) {
    path = getSpecialPath(edgePathParams, offset);
  } else {
    [path] = getBezierPath(edgePathParams);
  }

  const edgeStyle = {
    strokeWidth: 4,
  };

  return (
    <>
      <BaseEdge path={path} style={{ ...style, ...edgeStyle }} />
      <EdgeText
        x={(sourceX + targetX) / 2}
        y={(sourceY + targetY + offset) / 2}
        label={label}
        labelBgStyle={{ fill: '#F7F9FB' }}
        labelStyle={{ fill: 'black', fontWeight: 'bold', fontSize: '0.8em' }}
      />
    </>
  );
}
