import { BaseEdge, type EdgeProps, EdgeText, getBezierPath } from '@xyflow/react';

export type GetSpecialPathParams = {
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
};

// This is a custom edge because by default the xyflow library does not support multi-edges between two nodes.
export default function CustomEdge({
  source,
  target,
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

  // First edge has a normal bezier path, the rest have a special path that uses an offset to avoid overlapping.
  if (offset !== 0) {
    path = getSpecialPath(edgePathParams, offset);
  } else {
    [path] = getBezierPath(edgePathParams);
  }

  const flashyStyle = {
    strokeWidth: 5,
  };

  return (
    <>
      <BaseEdge path={path} style={{ ...style, ...flashyStyle }} />
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

export const getSpecialPath = ({ sourceX, sourceY, targetX, targetY }: GetSpecialPathParams, offset: number) => {
  const centerX = (sourceX + targetX) / 2;
  const centerY = (sourceY + targetY) / 2;

  return `M ${sourceX} ${sourceY} Q ${centerX} ${centerY + offset} ${targetX} ${targetY}`;
};
