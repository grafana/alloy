/**
 * renderSVG.ts — renders a LayoutResult from layoutGraph() to an SVG string.
 *
 * Styling intentionally mirrors the UI's ComponentGraph:
 *  - Light-coloured filled rectangles per component family
 *  - Colour-coded directed edges with arrowheads
 *  - Bold monospace type name + smaller instance label
 */

import type { LayoutResult } from '../../src/features/graph/layoutGraph.ts';

const NODE_HEIGHT = 112; // must match layoutGraph.ts

// Component family → [fill, stroke].
const FAMILY_COLORS: Record<string, [string, string]> = {
  otelcol: ['#DBEAFE', '#2563EB'],
  prometheus: ['#FEF3C7', '#D97706'],
  loki: ['#FEE2E2', '#DC2626'],
  pyroscope: ['#D1FAE5', '#059669'],
  discovery: ['#E0E7FF', '#4338CA'],
  remote: ['#F3E8FF', '#7C3AED'],
  local: ['#F1F5F9', '#475569'],
  mimir: ['#FFE4E6', '#E11D48'],
  faro: ['#FFF7ED', '#EA580C'],
};

const DEFAULT_COLORS: [string, string] = ['#F8FAFC', '#64748B'];

function familyColors(name: string): [string, string] {
  const prefix = name.split('.')[0];
  return FAMILY_COLORS[prefix] ?? DEFAULT_COLORS;
}

function xmlEscape(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

export function renderSVG(layout: LayoutResult): string {
  if (layout.nodes.length === 0) {
    const w = 300,
      h = 80;
    return (
      `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}">` +
      `<text x="${w / 2}" y="${h / 2}" text-anchor="middle" dominant-baseline="middle" ` +
      `font-family="sans-serif" font-size="14" fill="#64748B">No components found.</text>` +
      `</svg>`
    );
  }

  // Canvas bounds — tight fit with no extra padding.
  let maxX = 0,
    maxY = 0;
  for (const n of layout.nodes) {
    maxX = Math.max(maxX, n.x + n.width / 2);
    maxY = Math.max(maxY, n.y + NODE_HEIGHT / 2);
  }

  const lines: string[] = [];
  lines.push(
    `<svg xmlns="http://www.w3.org/2000/svg" ` +
      `width="${Math.ceil(maxX)}" height="${Math.ceil(maxY)}" ` +
      `viewBox="0 0 ${Math.ceil(maxX)} ${Math.ceil(maxY)}">`
  );

  // Arrowhead markers — one per colour family present in the edge set.
  const nodeById = new Map(layout.nodes.map((n) => [n.id, n]));
  const usedStrokes = new Set<string>();
  for (const e of layout.edges) {
    const src = nodeById.get(e.source);
    if (src) usedStrokes.add(familyColors(src.name)[1]);
  }

  lines.push('<defs>');
  for (const stroke of usedStrokes) {
    const id = `arrow-${stroke.replace('#', '')}`;
    lines.push(
      `  <marker id="${id}" markerWidth="10" markerHeight="7" refX="9" refY="3.5" orient="auto">` +
        `<polygon points="0 0, 10 3.5, 0 7" fill="${stroke}"/>` +
        `</marker>`
    );
  }
  lines.push('</defs>');


  // Edges (drawn beneath nodes).
  for (const e of layout.edges) {
    const src = nodeById.get(e.source);
    const dst = nodeById.get(e.target);
    if (!src || !dst) continue;

    const stroke = familyColors(src.name)[1];
    const markerId = `arrow-${stroke.replace('#', '')}`;

    // Right midpoint of source → left midpoint of target, cubic bezier.
    const x1 = src.x + src.width / 2;
    const y1 = src.y;
    const x2 = dst.x - dst.width / 2;
    const y2 = dst.y;
    const cx = (x1 + x2) / 2;

    lines.push(
      `<path d="M${f(x1)},${f(y1)} C${f(cx)},${f(y1)} ${f(cx)},${f(y2)} ${f(x2)},${f(y2)}" ` +
        `fill="none" stroke="${stroke}" stroke-width="1.5" marker-end="url(#${markerId})"/>`
    );
  }

  // Nodes.
  for (const n of layout.nodes) {
    const [fill, stroke] = familyColors(n.name);
    const x = n.x - n.width / 2;
    const y = n.y - NODE_HEIGHT / 2;
    const hasLabel = !!n.label;

    // Box.
    lines.push(
      `<rect x="${f(x)}" y="${f(y)}" width="${f(n.width)}" height="${NODE_HEIGHT}" ` +
        `rx="8" ry="8" fill="${fill}" stroke="${stroke}" stroke-width="1.5"/>`
    );

    // Component type name.
    const nameY = hasLabel ? y + NODE_HEIGHT * 0.38 : y + NODE_HEIGHT * 0.5;
    lines.push(
      `<text x="${f(n.x)}" y="${f(nameY)}" text-anchor="middle" dominant-baseline="middle" ` +
        `font-family="ui-monospace,monospace" font-size="21" font-weight="bold" fill="${stroke}">` +
        xmlEscape(n.name) +
        `</text>`
    );

    // Instance label.
    if (hasLabel) {
      lines.push(
        `<text x="${f(n.x)}" y="${f(y + NODE_HEIGHT * 0.7)}" text-anchor="middle" dominant-baseline="middle" ` +
          `font-family="ui-monospace,monospace" font-size="20" fill="#64748B">` +
          xmlEscape(`"${n.label}"`) +
          `</text>`
      );
    }
  }

  lines.push('</svg>');
  return lines.join('\n');
}

/** Format a float to 1 decimal place. */
function f(n: number): string {
  return n.toFixed(1);
}
