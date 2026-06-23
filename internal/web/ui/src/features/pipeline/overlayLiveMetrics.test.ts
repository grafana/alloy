import { type PipelineGraphData, SignalKind } from '@grafana/alloy-pipeline-graph';
import { describe, expect, it } from 'vitest';

import { type DebugData, DebugDataType } from './debugDataType';
import { overlayLiveMetrics } from './overlayLiveMetrics';

function graph(): PipelineGraphData {
  return {
    nodes: [],
    edges: [
      { id: 'a->b', source: 'a', target: 'b', signals: [SignalKind.Other] },
      { id: 'b->c', source: 'b', target: 'c', signals: [SignalKind.Other] },
    ],
  };
}

function debug(overrides: Partial<DebugData>): DebugData {
  return {
    componentID: 'a',
    targetComponentIDs: ['b'],
    rate: 1,
    type: DebugDataType.PROMETHEUS_METRIC,
    ...overrides,
  };
}

describe('overlayLiveMetrics', () => {
  it('groups multiple signal types on one edge into per-signal metrics', () => {
    const result = overlayLiveMetrics(graph(), [
      debug({ type: DebugDataType.PROMETHEUS_METRIC, rate: 100 }),
      debug({ type: DebugDataType.OTEL_LOG, rate: 50 }),
      debug({ type: DebugDataType.OTEL_TRACE, rate: 10 }),
    ]);

    const edge = result.edges.find((e) => e.id === 'a->b')!;
    expect(edge.signalMetrics).toEqual(
      expect.arrayContaining([
        { signal: SignalKind.Metrics, value: 100 },
        { signal: SignalKind.Logs, value: 50 },
        { signal: SignalKind.Traces, value: 10 },
      ])
    );
    expect(edge.signalMetrics).toHaveLength(3);
  });

  it('maps target debug data to the Targets signal', () => {
    const result = overlayLiveMetrics(graph(), [debug({ type: DebugDataType.TARGET, rate: 128 })]);

    const edge = result.edges.find((e) => e.id === 'a->b')!;
    expect(edge.signalMetrics).toEqual([{ signal: SignalKind.Targets, value: 128 }]);
  });

  it('sums rates of the same signal (prometheus + otel metrics)', () => {
    const result = overlayLiveMetrics(graph(), [
      debug({ type: DebugDataType.PROMETHEUS_METRIC, rate: 100 }),
      debug({ type: DebugDataType.OTEL_METRIC, rate: 25 }),
    ]);

    const edge = result.edges.find((e) => e.id === 'a->b')!;
    expect(edge.signalMetrics).toEqual([{ signal: SignalKind.Metrics, value: 125 }]);
  });

  it('ignores non-positive rates and clears stale metrics', () => {
    const seeded = graph();
    seeded.edges[0].signalMetrics = [{ signal: SignalKind.Metrics, value: 999 }];

    const result = overlayLiveMetrics(seeded, [debug({ rate: 0 })]);

    const edge = result.edges.find((e) => e.id === 'a->b')!;
    expect(edge.signalMetrics).toBeUndefined();
  });

  it('applies source-only debug data (no targets) to every outgoing edge', () => {
    const g: PipelineGraphData = {
      nodes: [],
      edges: [
        { id: 'a->b', source: 'a', target: 'b', signals: [SignalKind.Other] },
        { id: 'a->c', source: 'a', target: 'c', signals: [SignalKind.Other] },
      ],
    };

    const result = overlayLiveMetrics(g, [debug({ targetComponentIDs: [], rate: 7 })]);

    expect(result.edges.find((e) => e.id === 'a->b')!.signalMetrics).toEqual([{ signal: SignalKind.Metrics, value: 7 }]);
    expect(result.edges.find((e) => e.id === 'a->c')!.signalMetrics).toEqual([{ signal: SignalKind.Metrics, value: 7 }]);
  });

  it('matches inner (container/local) edge endpoints by localID', () => {
    const g: PipelineGraphData = {
      nodes: [],
      edges: [{ id: 'mod/a->mod/b', source: 'mod/a', target: 'mod/b', signals: [SignalKind.Other] }],
    };

    const result = overlayLiveMetrics(g, [debug({ componentID: 'mod/a', targetComponentIDs: ['mod/b'], rate: 42 })]);

    expect(result.edges[0].signalMetrics).toEqual([{ signal: SignalKind.Metrics, value: 42 }]);
  });
});
