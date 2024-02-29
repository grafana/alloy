import { FC, useEffect, useRef } from 'react';
import { useHref } from 'react-router-dom';
import * as d3 from 'd3';
import { coordSimplex, dagStratify, decrossTwoLayer, layeringCoffmanGraham, NodeSizeAccessor, sugiyama } from 'd3-dag';
import { Point } from 'd3-dag/dist/dag';
import { IdOperator, ParentIdsOperator } from 'd3-dag/dist/dag/create';
import * as d3Zoom from 'd3-zoom';

import { ComponentHealthState, ComponentInfo } from '../component/types';

let canvas: HTMLCanvasElement | undefined;

/**
 * calcTextWidth calculates the width of text if it were to be rendered on
 * screen.
 *
 * font should be a font specifier like "bold 16pt arial"
 */
function calcTextWidth(text: string, font: string): number | null {
  // Adapted from https://stackoverflow.com/a/21015393

  // Lazy-load the canvas if it hasn't been created yet.
  if (canvas === undefined) {
    canvas = document.createElement('canvas');
  }

  const context = canvas.getContext('2d');
  if (context == null) {
    return null;
  }
  context.font = font;
  return context.measureText(text).width;
}

/**
 * intersectsBox reports whether a point intersects a box.
 */
function intersectsBox(point: Point, box: Box): boolean {
  return (
    point.x >= box.x && // after starting X
    point.y >= box.y && // after starting Y
    point.x <= box.x + box.w && // before ending X
    point.y <= box.y + box.h // before ending Y
  );
}

interface Line {
  start: Point;
  end: Point;
}

/*
 * boxIntersectionPoint returns the point where line intersects box.
 */
function boxIntersectionPoint(line: Line, box: Box): Point {
  const boxTop: Line = { start: { x: box.x, y: box.y }, end: { x: box.x + box.w, y: box.y } };
  const topIntersectionPoint = lineIntersectionPoint(line, boxTop);
  if (topIntersectionPoint !== undefined) {
    return topIntersectionPoint;
  }

  const boxRight: Line = { start: { x: box.x + box.w, y: box.y }, end: { x: box.x + box.w, y: box.y + box.h } };
  const rightIntersectionPoint = lineIntersectionPoint(line, boxRight);
  if (rightIntersectionPoint !== undefined) {
    return rightIntersectionPoint;
  }

  const boxBottom: Line = { start: { x: box.x, y: box.y + box.h }, end: { x: box.x + box.w, y: box.y + box.h } };
  const bottomIntersectionPoint = lineIntersectionPoint(line, boxBottom);
  if (bottomIntersectionPoint !== undefined) {
    return bottomIntersectionPoint;
  }

  const boxLeft: Line = { start: { x: box.x, y: box.y }, end: { x: box.x, y: box.y + box.h } };
  const leftInsersectionPoint = lineIntersectionPoint(line, boxLeft);
  if (leftInsersectionPoint !== undefined) {
    return leftInsersectionPoint;
  }

  // No intersection; just return the last point of the line.
  return line.end;
}

/*
 * lineIntersectionPoint returns the point where l1 and l2 intersect.
 *
 * Returns undefined if the lines do not intersect.
 */
function lineIntersectionPoint(l1: Line, l2: Line): Point | undefined {
  // https://en.wikipedia.org/wiki/Line%E2%80%93line_intersection#Given_two_points_on_each_line_segment

  // l1 = (x1, y1) -> (x2, y2)
  // l2 = (x3, y3) -> (x4, y4)
  const [x1, y1] = [l1.start.x, l1.start.y];
  const [x2, y2] = [l1.end.x, l1.end.y];
  const [x3, y3] = [l2.start.x, l2.start.y];
  const [x4, y4] = [l2.end.x, l2.end.y];

  const denominator = (x1 - x2) * (y3 - y4) - (y1 - y2) * (x3 - x4);
  if (denominator === 0) {
    return undefined;
  }

  const t_numerator = (x1 - x3) * (y3 - y4) - (y1 - y3) * (x3 - x4);
  const u_numerator = (x1 - x3) * (y1 - y2) - (y1 - y3) * (x1 - x2);

  // Only t is used for calculating the point, but both t and u must be defined
  // to ensure the intersection exists.
  const [t, u] = [t_numerator / denominator, u_numerator / denominator];

  // There is an intersection if and only if 0 <= t <= 1 and 0 <= u <= 1
  if (0 <= t && t <= 1 && 0 <= u && u <= 1) {
    return {
      x: x1 + t * (x2 - x1),
      y: y1 + t * (y2 - y1),
    };
  }

  return undefined;
}

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface ComponentGraphProps {
  components: ComponentInfo[];
}

/**
 * ComponentGraph renders an SVG with relationships between defined components.
 * The components prop must be a non-empty array.
 */
export const ComponentGraph: FC<ComponentGraphProps> = (props) => {
  const baseComponentPath = useHref('/component');
  const svgRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    // NOTE(rfratto): The default units of svg are in pixels.

    const [nodeWidth, nodeHeight] = [150, 75];
    const nodeMargin = 25;
    const nodePadding = 5;

    const contentHeight = nodeHeight - nodePadding * 2;

    const widthCache: Record<string, number> = {
      foo: 5,
    };

    const builder = dagStratify()
      .id<IdOperator<ComponentInfo>>((n) => n.localID)
      .parentIds<ParentIdsOperator<ComponentInfo>>((n) => n.referencedBy);
    const dag = builder(props.components);

    // Our graph layout is optimized for graphs of 50 components or more. The
    // decross method is where most of the layout time is spent; decrossOpt is
    // far too slow.
    //
    // We also use Coffman Graham for layering, which constrains the final
    // width of the graph as much as possible.
    const layout = sugiyama()
      .layering(layeringCoffmanGraham())
      .decross(decrossTwoLayer())
      .coord(coordSimplex())
      .nodeSize<NodeSizeAccessor<ComponentInfo, undefined>>((n) => {
        // nodeSize is the full amount of space you want the node to take up.
        //
        // It can be considered similar to the box model: margin and padding should
        // be added to the size.

        // n will be undefined for synthetic nodes in a layer. These synthetic
        // nodes can be given sizes, but we keep them at [0, 0] to minimize the
        // total width of the graph.
        if (n === undefined) {
          return [0, 0];
        }

        // Calculate how much width the text needs to be displayed.
        let width = nodeWidth;

        const displayFont = "bold 13px 'Roboto', sans-serif";

        const nameWidth = calcTextWidth(n.data.name, displayFont);
        if (nameWidth != null && nameWidth > width) {
          width = nameWidth;
        }

        const labelWidth = calcTextWidth(n.data.label || '', displayFont);
        if (labelWidth != null && labelWidth > width) {
          width = labelWidth;
        }

        // Cache the width so it can be used while plotting the SVG.
        widthCache[n.data.localID] = width;

        return [width + nodeMargin + nodePadding * 2, nodeHeight + nodeMargin + nodePadding * 2];
      });
    const { width, height } = layout(dag);

    // svgRef.current needs to be cast to an Element for type checks to work.
    // SVGSVGElement doesn't extend element and prevents zoom from
    // typechecking.
    //
    // Everything still seems to work even with the type cast.
    const svgSelection = d3.select(svgRef.current as Element);
    svgSelection.selectAll('*').remove(); // Clear svg content before adding new elements
    svgSelection.attr('viewBox', [0, 0, width, height].join(' '));

    const svgWrapper = svgSelection.append('g');

    // TODO(rfratto): determine a reasonable zoom scale extent based on size of
    // layout rather than hard coding 0.1x to 10x.
    //
    // As it is now, you can zoom in way too close on really small graphs.
    const zoom = d3Zoom
      .zoom()
      .scaleExtent([0.1, 10])
      .on('zoom', (e) => {
        svgWrapper.attr('transform', e.transform);
      });

    svgSelection.call(zoom).call(zoom.transform, d3Zoom.zoomIdentity);

    // Add a marker element so we can draw an arrow pointing between nodes.
    svgWrapper
      .append('defs')
      .append('marker')
      .attr('id', 'arrow')
      .attr('viewBox', [0, 0, 20, 20])
      .attr('refX', 17)
      .attr('refY', 10)
      .attr('markerWidth', 5)
      .attr('markerHeight', 5)
      .attr('orient', 'auto-start-reverse')
      .append('path')
      .attr(
        'd',
        // Draw an arrow shape
        d3.line()([
          [0, 0], // Bottom left of arrow
          [0, 20], // Top left of arrow
          [20, 10], // Middle point of arrow
        ])
      )
      .attr('fill', '#c8c9ca');

    const line = d3
      .line<Point>()
      .curve(d3.curveCatmullRom)
      .x((d) => d.x)
      .y((d) => d.y);

    // Plot edges
    svgWrapper
      .append('g')
      .selectAll('path')
      .data(dag.links())
      .enter()
      .append('path')
      .attr('marker-end', 'url(#arrow)')
      .attr('d', (node) => {
        // We want to draw arrows between boxes, but by default the arrows are
        // obscured; d3-dag points lines to the middle of a box which is hidden
        // by the rectangle.
        //
        // To fix this, we do the following:
        //
        // 1. Retrieve the set of generated points for d3-dag
        // 2. Remove all points after the first point which intersects the box
        // 3. Move the final point to the coordinates where it intersects the
        //    box
        // 4. The line will now stop at the box edge as expected.

        const nodeBox: Box = {
          x: (node.target.x || 0) - widthCache[node.target.data.localID] / 2 - nodePadding,
          y: (node.target.y || 0) - nodeHeight / 2 - nodePadding,
          w: widthCache[node.target.data.localID] + nodePadding * 2,
          h: nodeHeight + nodePadding * 2,
        };

        const idx = node.points.findIndex((p) => {
          return intersectsBox(p, nodeBox);
        });
        if (idx === -1) {
          // It shouldn't be possible for this to happen; we know that the
          // final point always goes to the center of the target box so there
          // should always be an intersection.
          throw new Error('could not find point of intersection with target node');
        }
        const trimmedPoints = node.points.slice(0, idx + 1);

        const intersectingLine = {
          start: trimmedPoints[trimmedPoints.length - 2],
          end: trimmedPoints[trimmedPoints.length - 1],
        };
        const fixedPoint = boxIntersectionPoint(intersectingLine, nodeBox);
        trimmedPoints[trimmedPoints.length - 1] = fixedPoint;

        return line(trimmedPoints);
      })
      .attr('fill', 'none')
      .attr('stroke-width', '2px')
      .attr('stroke', '#c8c9ca')
      .append('title') // Append tooltip to edge
      .text((n) => {
        return `${n.source.data.localID} to ${n.target.data.localID}`;
      });

    // Select nodes
    const nodes = svgWrapper
      .append('g')
      .selectAll('g')
      .data(dag.descendants())
      .enter()
      .append('g')
      .attr('transform', (node) => {
        // node.x, node.y refer to the absolute center of the box.
        //
        // We translate the group to the top-left corner to make it easier to
        // position all the elements. Top left corner should account for
        // padding space.
        const x = (node.x || 0) - widthCache[node.data.localID] / 2 - nodePadding;
        const y = (node.y || 0) - nodeHeight / 2 - nodePadding;
        return `translate(${x}, ${y})`;
      });

    const linkedNodes = nodes.append('a').attr('href', (n) => `${baseComponentPath}/${n.data.localID}`);

    // Plot nodes
    linkedNodes
      .append('rect')
      .attr('fill', '#f2f2f3')
      .attr('rx', 3)
      .attr('height', nodeHeight + nodePadding * 2)
      .attr('width', (node) => {
        return widthCache[node.data.localID] + nodePadding * 2;
      })
      .attr('stroke-width', '1')
      .attr('stroke', '#e4e5e6');

    // Create a group for node content which is anchored inside of the padding
    // area.
    const nodeContent = linkedNodes.append('g').attr('transform', `translate(${nodePadding}, ${nodePadding})`);

    // Add component name text
    nodeContent
      .append('text')
      .text((d) => d.data.name)
      .attr('font-size', '13')
      .attr('font-weight', 'bold')
      .attr('font-family', '"Roboto", sans-serif')
      .attr('text-anchor', 'start')
      .attr('alignment-baseline', 'hanging')
      .attr('fill', 'rgb(36, 41, 46, 0.75)');

    // Add component label text
    nodeContent
      .append('text')
      .text((d) => d.data.label || '')
      .attr('y', 13 /* font size */ + 2 /* margin from previous text line */)
      .attr('font-size', '13')
      .attr('font-weight', 'normal')
      .attr('font-family', '"Roboto", sans-serif')
      .attr('text-anchor', 'start')
      .attr('alignment-baseline', 'hanging')
      .attr('fill', 'rgb(36, 41, 46, 0.75)');

    // Draw health status
    const healthBox = nodeContent
      .append('g')
      .attr('transform', `translate(0, ${contentHeight - 3})`); /* 1/4 height (why?) */

    healthBox
      .append('rect')
      .attr('fill', (node) => {
        switch (node.data.health.state || ComponentHealthState.UNKNOWN) {
          case ComponentHealthState.HEALTHY:
            return '#3b8160';
          case ComponentHealthState.UNHEALTHY:
            return '#d2476d';
          case ComponentHealthState.EXITED:
            return '#d2476d';
          case ComponentHealthState.UNKNOWN:
            return '#f5d65b';
        }
      })
      .attr('rx', 1)
      .attr('height', 14)
      .attr('width', 45);

    healthBox
      .append('text')
      .text((d) => {
        const text = d.data.health.state || 'unknown';
        return text.charAt(0).toUpperCase() + text.substring(1);
      })
      .attr('x', 45 / 2) // Anchor to middle of box
      .attr('y', 14 / 2) // Middle of box
      .attr('font-size', '7')
      .attr('font-weight', 'bold')
      .attr('font-family', '"Roboto", sans-serif')
      .attr('text-anchor', 'middle')
      .attr('alignment-baseline', 'middle')
      .attr('fill', (node) => {
        if (node.data.health.state === ComponentHealthState.UNKNOWN) {
          return '#000000';
        }
        return '#ffffff';
      });
  });

  return <svg ref={svgRef} style={{ width: '100%', height: '100%', display: 'block' }} />;
};
