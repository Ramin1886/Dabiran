# @git-viz/wasm-math

Rust → WebAssembly canvas math engine for the visualization frontend. It
offloads the per-frame geometry on large graphs to wasm and returns packed
coordinate arrays the renderer feeds straight into the WebGL line buffer
(see `docs/tech_stack.md` §3).

## Exports

| Function | Input | Output | Purpose |
| :--- | :--- | :--- | :--- |
| `cull_indices(positions, minX, minY, maxX, maxY)` | `Float32Array` `[x0,y0,…]` | `Uint32Array` | Indices of nodes inside the viewport rect |
| `cull_segment_indices(segments, minX, minY, maxX, maxY)` | `Float32Array` `[ax,ay,bx,by,…]` | `Uint32Array` | Indices of connector segments touching the rect |
| `bezier_polyline(sx, sy, ex, ey, segments)` | floats | `Float32Array` `[x0,y0,…]` | Flattened branch connector (straight within a lane, cubic Bezier across lanes) |

The default export is the async `init()` from `wasm-pack --target web`.

## Layout

- `src/core.rs` — pure, dependency-free math, unit-tested natively. The
  TypeScript fallback in `apps/frontend/src/math/engine.ts` mirrors it exactly.
- `src/lib.rs` — thin `wasm-bindgen` wrappers over `core`.
- `pkg/` — committed `wasm-pack` build output so the frontend builds without a
  Rust toolchain.

## Build & test

```bash
# from this directory (requires rustup + wasm32-unknown-unknown + wasm-pack)
cargo test               # native unit tests of the core math
npm run build:wasm       # regenerate pkg/ (wasm-pack build --target web)
```

Regenerate `pkg/` and commit it whenever `src/` changes. The root convenience
script `npm run build:wasm` builds this package from the repo root.
