/**
 * Canvas math engine: viewport culling and Bezier spline flattening.
 *
 * The hot per-frame geometry is offloaded to the Rust→WebAssembly module
 * `@git-viz/wasm-math` once it has initialized; until then (and if the wasm
 * fails to load, e.g. in a non-browser test environment) an equivalent pure
 * TypeScript fallback is used. The fallback mirrors `packages/wasm-math/src/
 * core.rs` exactly, so results are identical whichever path runs — the wasm is
 * purely an acceleration for large graphs.
 *
 * All exported geometry functions are synchronous so the renderer can call
 * them inside a memo/draw pass; {@link initMathEngine} flips the backing
 * implementation to wasm asynchronously when it becomes available.
 */

/** Axis-aligned world-space rectangle used for viewport culling. */
export interface WorldRect {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

/** The subset of the wasm module the engine uses. */
interface WasmMath {
  default: (input?: unknown) => Promise<unknown>;
  cull_indices: (
    positions: Float32Array,
    minX: number,
    minY: number,
    maxX: number,
    maxY: number,
  ) => Uint32Array;
  cull_segment_indices: (
    segments: Float32Array,
    minX: number,
    minY: number,
    maxX: number,
    maxY: number,
  ) => Uint32Array;
  bezier_polyline: (
    sx: number,
    sy: number,
    ex: number,
    ey: number,
    segments: number,
  ) => Float32Array;
}

let wasm: WasmMath | null = null;
let initPromise: Promise<boolean> | null = null;

/**
 * Lazily loads and initializes the wasm math module. Safe to call repeatedly:
 * the first call performs the work, later calls return the same result. Any
 * failure (module not built, no WebAssembly/fetch in the environment) is
 * swallowed and leaves the engine on its TypeScript fallback.
 *
 * @returns true when wasm is active, false when running on the fallback
 */
export function initMathEngine(): Promise<boolean> {
  if (initPromise) return initPromise;
  initPromise = (async () => {
    try {
      const mod = (await import('@git-viz/wasm-math')) as unknown as WasmMath;
      await mod.default();
      wasm = mod;
      return true;
    } catch {
      wasm = null;
      return false;
    }
  })();
  return initPromise;
}

/** True when the wasm backend is active (false on the TS fallback). */
export function isWasmActive(): boolean {
  return wasm !== null;
}

/** Resets engine state. Test-only helper. */
export function __resetMathEngineForTest(): void {
  wasm = null;
  initPromise = null;
}

// --- Pure TypeScript fallback (mirrors packages/wasm-math/src/core.rs) -------

/** Tests whether a world point falls inside a rectangle (inclusive bounds). */
export function pointInRect(x: number, y: number, r: WorldRect): boolean {
  return x >= r.minX && x <= r.maxX && y >= r.minY && y <= r.maxY;
}

/**
 * Tests whether the segment (ax,ay)-(bx,by) is relevant to the rect: relevant
 * when either endpoint is inside, or when the segment's bounding box overlaps
 * the rect (the cheap conservative crossing test that keeps long diagonals
 * spanning the viewport drawn).
 */
export function segmentTouchesRect(
  ax: number,
  ay: number,
  bx: number,
  by: number,
  r: WorldRect,
): boolean {
  if (pointInRect(ax, ay, r) || pointInRect(bx, by, r)) return true;
  const segMinX = Math.min(ax, bx);
  const segMaxX = Math.max(ax, bx);
  const segMinY = Math.min(ay, by);
  const segMaxY = Math.max(ay, by);
  return segMaxX >= r.minX && segMinX <= r.maxX && segMaxY >= r.minY && segMinY <= r.maxY;
}

/** TS fallback for `cull_indices`. */
export function cullIndicesJs(positions: Float32Array, r: WorldRect): Uint32Array {
  const n = Math.floor(positions.length / 2);
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    if (pointInRect(positions[2 * i], positions[2 * i + 1], r)) out.push(i);
  }
  return Uint32Array.from(out);
}

/** TS fallback for `cull_segment_indices`. */
export function cullSegmentIndicesJs(segments: Float32Array, r: WorldRect): Uint32Array {
  const m = Math.floor(segments.length / 4);
  const out: number[] = [];
  for (let i = 0; i < m; i++) {
    const o = 4 * i;
    if (segmentTouchesRect(segments[o], segments[o + 1], segments[o + 2], segments[o + 3], r)) {
      out.push(i);
    }
  }
  return Uint32Array.from(out);
}

/** TS fallback for `bezier_polyline`. */
export function bezierPolylineJs(
  sx: number,
  sy: number,
  ex: number,
  ey: number,
  segments: number,
): Float32Array {
  if (sy === ey) return Float32Array.of(sx, sy, ex, ey);
  const steps = Math.max(1, segments);
  const cpx = sx + (ex - sx) / 2;
  const out = new Float32Array((steps + 1) * 2);
  for (let i = 0; i <= steps; i++) {
    const t = i / steps;
    const mt = 1 - t;
    const b0 = mt * mt * mt;
    const b1 = 3 * mt * mt * t;
    const b2 = 3 * mt * t * t;
    const b3 = t * t * t;
    out[2 * i] = b0 * sx + b1 * cpx + b2 * cpx + b3 * ex;
    out[2 * i + 1] = b0 * sy + b1 * sy + b2 * ey + b3 * ey;
  }
  return out;
}

// --- Public geometry API (wasm when active, else TS fallback) ----------------

/** Indices of nodes (packed `[x0,y0,…]`) inside the rect. */
export function cullIndices(positions: Float32Array, r: WorldRect): Uint32Array {
  if (wasm) return wasm.cull_indices(positions, r.minX, r.minY, r.maxX, r.maxY);
  return cullIndicesJs(positions, r);
}

/** Indices of connector segments (packed `[ax,ay,bx,by,…]`) touching the rect. */
export function cullSegmentIndices(segments: Float32Array, r: WorldRect): Uint32Array {
  if (wasm) return wasm.cull_segment_indices(segments, r.minX, r.minY, r.maxX, r.maxY);
  return cullSegmentIndicesJs(segments, r);
}

/** Flattened branch connector polyline (`[x0,y0,…]`). */
export function bezierPolyline(
  sx: number,
  sy: number,
  ex: number,
  ey: number,
  segments: number,
): Float32Array {
  if (wasm) return wasm.bezier_polyline(sx, sy, ex, ey, segments);
  return bezierPolylineJs(sx, sy, ex, ey, segments);
}
