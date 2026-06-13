import { describe, it, expect, vi, beforeEach } from 'vitest';

// Hoisted fakes so the vi.mock factory can reference them.
const { fakeInit, fakeCullIndices, fakeCullSeg, fakeBezier, fakeLayout } = vi.hoisted(() => ({
  fakeInit: vi.fn(),
  fakeCullIndices: vi.fn(),
  fakeCullSeg: vi.fn(),
  fakeBezier: vi.fn(),
  fakeLayout: vi.fn(),
}));

vi.mock('@git-viz/wasm-math', () => ({
  default: fakeInit,
  cull_indices: fakeCullIndices,
  cull_segment_indices: fakeCullSeg,
  bezier_polyline: fakeBezier,
  layout: fakeLayout,
}));

import {
  __resetMathEngineForTest,
  initMathEngine,
  isWasmActive,
  cullIndices,
  cullSegmentIndices,
  bezierPolyline,
  layoutNodes,
  cullIndicesJs,
  cullSegmentIndicesJs,
  bezierPolylineJs,
  layoutNodesJs,
  pointInRect,
  segmentTouchesRect,
} from './engine';

const rect = { minX: 0, minY: 0, maxX: 100, maxY: 100 };

beforeEach(() => {
  __resetMathEngineForTest();
  fakeInit.mockReset().mockResolvedValue({});
  fakeCullIndices.mockReset();
  fakeCullSeg.mockReset();
  fakeBezier.mockReset();
  fakeLayout.mockReset();
});

describe('pure TS fallback geometry (mirrors wasm-math core.rs)', () => {
  it('pointInRect is inclusive', () => {
    expect(pointInRect(0, 0, rect)).toBe(true);
    expect(pointInRect(100, 100, rect)).toBe(true);
    expect(pointInRect(-1, 50, rect)).toBe(false);
  });

  it('segmentTouchesRect: endpoint inside, bbox overlap, or disjoint', () => {
    expect(segmentTouchesRect(50, 50, 500, 500, rect)).toBe(true); // endpoint inside
    expect(segmentTouchesRect(-50, 50, 500, 50, rect)).toBe(true); // bbox spans
    expect(segmentTouchesRect(200, 200, 300, 300, rect)).toBe(false); // disjoint
  });

  it('cullIndicesJs keeps only points inside the rect', () => {
    const positions = Float32Array.of(10, 10, 200, 10, 90, 90);
    expect(Array.from(cullIndicesJs(positions, rect))).toEqual([0, 2]);
  });

  it('cullSegmentIndicesJs filters segments by touch', () => {
    const segs = Float32Array.of(50, 50, 500, 500, 200, 200, 300, 300);
    expect(Array.from(cullSegmentIndicesJs(segs, rect))).toEqual([0]);
  });

  it('bezierPolylineJs returns a straight 2-point line within a lane', () => {
    expect(Array.from(bezierPolylineJs(10, 40, 90, 40, 8))).toEqual([10, 40, 90, 40]);
  });

  it('bezierPolylineJs samples a cubic Bezier with matching endpoints', () => {
    const segments = 8;
    const poly = bezierPolylineJs(0, 0, 100, 80, segments);
    expect(poly.length).toBe((segments + 1) * 2);
    expect(poly[0]).toBeCloseTo(0);
    expect(poly[1]).toBeCloseTo(0);
    expect(poly[poly.length - 2]).toBeCloseTo(100);
    expect(poly[poly.length - 1]).toBeCloseTo(80);
    // X stays within the span by control-point symmetry.
    for (let k = 0; k < poly.length; k += 2) {
      expect(poly[k]).toBeGreaterThanOrEqual(-0.01);
      expect(poly[k]).toBeLessThanOrEqual(100.01);
    }
  });

  it('bezierPolylineJs clamps zero segments to a single step', () => {
    expect(bezierPolylineJs(0, 0, 10, 10, 0).length).toBe(4);
  });

  it('layoutNodesJs assigns lanes (parent takeover, new lane) and offsets', () => {
    // A(t=0,no parent), B(t=10,parent A=idx0), C(t=20,no parent).
    const out = Array.from(
      layoutNodesJs(Float64Array.of(0, 10, 20), Int32Array.of(-1, 0, -1)),
    );
    // [laneA,xA, laneB,xB, laneC,xC]
    expect(out).toEqual([0, 0, 0, 0.5, 1, 1]);
  });

  it('layoutNodesJs anchors the origin at the oldest date regardless of order', () => {
    const out = Array.from(layoutNodesJs(Float64Array.of(20, 0), Int32Array.of(-1, -1)));
    expect(out[3]).toBeCloseTo(0); // node 1 (t=0) is origin
    expect(out[1]).toBeCloseTo(1); // node 0 (t=20) -> 20*0.05
    expect(out[0]).not.toEqual(out[2]); // distinct lanes for two roots
  });

  it('layoutNodesJs handles an empty set', () => {
    expect(layoutNodesJs(new Float64Array(0), new Int32Array(0)).length).toBe(0);
  });
});

describe('engine backend selection', () => {
  it('uses the TS fallback before the wasm engine initializes', () => {
    expect(isWasmActive()).toBe(false);
    const positions = Float32Array.of(10, 10, 200, 10);
    expect(Array.from(cullIndices(positions, rect))).toEqual([0]);
    expect(fakeCullIndices).not.toHaveBeenCalled();
  });

  it('delegates to wasm once initialized', async () => {
    fakeCullIndices.mockReturnValue(Uint32Array.of(42));
    fakeCullSeg.mockReturnValue(Uint32Array.of(7));
    fakeBezier.mockReturnValue(Float32Array.of(1, 2, 3, 4));
    fakeLayout.mockReturnValue(Float32Array.of(5, 6));

    const ready = await initMathEngine();
    expect(ready).toBe(true);
    expect(isWasmActive()).toBe(true);

    expect(Array.from(cullIndices(Float32Array.of(0, 0), rect))).toEqual([42]);
    expect(Array.from(cullSegmentIndices(Float32Array.of(0, 0, 1, 1), rect))).toEqual([7]);
    expect(Array.from(bezierPolyline(0, 0, 1, 1, 4))).toEqual([1, 2, 3, 4]);
    expect(Array.from(layoutNodes(Float64Array.of(0), Int32Array.of(-1)))).toEqual([5, 6]);
    expect(fakeCullIndices).toHaveBeenCalledWith(expect.any(Float32Array), 0, 0, 100, 100);
  });

  it('stays on the fallback when wasm init fails', async () => {
    fakeInit.mockRejectedValueOnce(new Error('no WebAssembly here'));
    const ready = await initMathEngine();
    expect(ready).toBe(false);
    expect(isWasmActive()).toBe(false);
    // Falls back to the identical TS result.
    const positions = Float32Array.of(10, 10, 200, 10);
    expect(Array.from(cullIndices(positions, rect))).toEqual([0]);
    expect(fakeCullIndices).not.toHaveBeenCalled();
  });

  it('initMathEngine is idempotent (single attempt)', async () => {
    await initMathEngine();
    await initMathEngine();
    expect(fakeInit).toHaveBeenCalledTimes(1);
  });
});
