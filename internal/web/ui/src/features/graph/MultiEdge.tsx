import React from 'react';
import { BaseEdge, type EdgeProps, EdgeText, getBezierPath, type ReactFlowState, useStore } from '@xyflow/react';

export type GetSpecialPathParams = {
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
};

export const getSpecialPath = ({ sourceX, sourceY, targetX, targetY }: GetSpecialPathParams, offset: number) => {
  const centerX = (sourceX + targetX) / 2;
  const centerY = (sourceY + targetY) / 2;

  return `M ${sourceX} ${sourceY} Q ${centerX} ${centerY + offset} ${targetX} ${targetY}`;
};

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
  interactionWidth,
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

  const offset = interactionWidth ? interactionWidth : 0;

  if (offset > 0) {
    // If there are multiple edges, create a unique path with the offset
    path = getSpecialPath(edgePathParams, offset);
  } else {
    // For the first edge, use the default bezier path
    [path] = getBezierPath(edgePathParams);
  }

  return (
    <>
      <BaseEdge path={path} style={style} />
      <EdgeText
        x={(sourceX + targetX) / 2}
        y={(sourceY + targetY + offset) / 2}
        label={label}
        labelStyle={{ fill: 'black' }}
      />
    </>
  );
}
