import type { ComponentInfo } from '../component/types';

const AVERAGE_CHAR_WIDTH = 7.5;
const PADDING = 30;
const MIN_WIDTH = 120;
const MAX_WIDTH = 600;
const SHORT_THRESHOLD = 40;

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n));
}

/**
 * Computes a node width based on the longer of the name/label segments,
 * or a concat of both if the total length is below a certain
 * threshold. The results are clamped to "reasonable" bounds.
 */
export function estimatedWidthOfNode(component: ComponentInfo): number {
  const nameLen = component.name.length;
  const labelLen = component.label?.length ?? 0;
  const totalLen = nameLen + labelLen;
  const longestLen = Math.max(nameLen, labelLen);

  // For short combined strings, prefer single-line sizing; otherwise use the longest segment.
  const basisLen = totalLen < SHORT_THRESHOLD ? totalLen : longestLen;
  const raw = basisLen * AVERAGE_CHAR_WIDTH + PADDING;
  return Math.round(clamp(raw, MIN_WIDTH, MAX_WIDTH));
}
