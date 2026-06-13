//! WebAssembly canvas math engine for the Git visualization frontend.
//!
//! Exposes viewport culling and Bezier spline flattening over packed
//! `Float32Array`/`Uint32Array` buffers so the frontend can offload the
//! per-frame geometry on large graphs (tens of thousands of nodes) to wasm and
//! feed the resulting coordinate arrays straight into the WebGL line buffer
//! (see docs/tech_stack.md §3). All real logic lives in the dependency-free
//! [`core`] module, which is unit-tested natively; these wrappers are thin
//! shims. The TypeScript fallback mirrors `core` exactly.

mod core;

use wasm_bindgen::prelude::*;

/// Returns the indices of nodes (packed `[x0,y0,x1,y1,...]`) inside the
/// rectangle, as a `Uint32Array`.
#[wasm_bindgen]
pub fn cull_indices(positions: &[f32], min_x: f32, min_y: f32, max_x: f32, max_y: f32) -> Vec<u32> {
    core::cull_point_indices(positions, core::Rect { min_x, min_y, max_x, max_y })
}

/// Returns the indices of connector segments (packed `[ax,ay,bx,by,...]`) that
/// touch the rectangle, as a `Uint32Array`.
#[wasm_bindgen]
pub fn cull_segment_indices(
    segments: &[f32],
    min_x: f32,
    min_y: f32,
    max_x: f32,
    max_y: f32,
) -> Vec<u32> {
    core::cull_segment_indices(segments, core::Rect { min_x, min_y, max_x, max_y })
}

/// Flattens a branch connector into a polyline (`Float32Array` of
/// `[x0,y0,...]`): a straight segment within a lane, or a sampled cubic Bezier
/// across lanes. See [`core::bezier_polyline`].
#[wasm_bindgen]
pub fn bezier_polyline(sx: f32, sy: f32, ex: f32, ey: f32, segments: u32) -> Vec<f32> {
    core::bezier_polyline(sx, sy, ex, ey, segments)
}
