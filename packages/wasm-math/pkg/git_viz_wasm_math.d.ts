/* tslint:disable */
/* eslint-disable */

/**
 * Flattens a branch connector into a polyline (`Float32Array` of
 * `[x0,y0,...]`): a straight segment within a lane, or a sampled cubic Bezier
 * across lanes. See [`core::bezier_polyline`].
 */
export function bezier_polyline(sx: number, sy: number, ex: number, ey: number, segments: number): Float32Array;

/**
 * Returns the indices of nodes (packed `[x0,y0,x1,y1,...]`) inside the
 * rectangle, as a `Uint32Array`.
 */
export function cull_indices(positions: Float32Array, min_x: number, min_y: number, max_x: number, max_y: number): Uint32Array;

/**
 * Returns the indices of connector segments (packed `[ax,ay,bx,by,...]`) that
 * touch the rectangle, as a `Uint32Array`.
 */
export function cull_segment_indices(segments: Float32Array, min_x: number, min_y: number, max_x: number, max_y: number): Uint32Array;

export type InitInput = RequestInfo | URL | Response | BufferSource | WebAssembly.Module;

export interface InitOutput {
    readonly memory: WebAssembly.Memory;
    readonly bezier_polyline: (a: number, b: number, c: number, d: number, e: number) => [number, number];
    readonly cull_indices: (a: number, b: number, c: number, d: number, e: number, f: number) => [number, number];
    readonly cull_segment_indices: (a: number, b: number, c: number, d: number, e: number, f: number) => [number, number];
    readonly __wbindgen_externrefs: WebAssembly.Table;
    readonly __wbindgen_free: (a: number, b: number, c: number) => void;
    readonly __wbindgen_malloc: (a: number, b: number) => number;
    readonly __wbindgen_start: () => void;
}

export type SyncInitInput = BufferSource | WebAssembly.Module;

/**
 * Instantiates the given `module`, which can either be bytes or
 * a precompiled `WebAssembly.Module`.
 *
 * @param {{ module: SyncInitInput }} module - Passing `SyncInitInput` directly is deprecated.
 *
 * @returns {InitOutput}
 */
export function initSync(module: { module: SyncInitInput } | SyncInitInput): InitOutput;

/**
 * If `module_or_path` is {RequestInfo} or {URL}, makes a request and
 * for everything else, calls `WebAssembly.instantiate` directly.
 *
 * @param {{ module_or_path: InitInput | Promise<InitInput> }} module_or_path - Passing `InitInput` directly is deprecated.
 *
 * @returns {Promise<InitOutput>}
 */
export default function __wbg_init (module_or_path?: { module_or_path: InitInput | Promise<InitInput> } | InitInput | Promise<InitInput>): Promise<InitOutput>;
