# Technology Stack

Stack choices are driven by three requirements: high-concurrency WebSocket
fan-out, deterministic massive-graph processing, and strict typing across the
client/server boundary.

## Summary Matrix

| Layer | Technology | Status |
| :--- | :--- | :--- |
| Backend API, git engine, WS relay | Go (net/http, go-git, gorilla/websocket, pgx) | Implemented |
| Frontend application | TypeScript, React, Vite | Implemented |
| WebGL rendering | PixiJS v7 with @pixi/react | Implemented |
| Client state | Zustand | Implemented |
| CRDT synchronization | Yjs + y-websocket | Implemented |
| Persistence | PostgreSQL (pgx/v5) | Implemented |
| Shared contracts | TypeScript workspace packages | Implemented |
| Containers / orchestration | Podman (Compose), Kubernetes (Kustomize) | Implemented |
| Full-text search | Meilisearch | Implemented |
| AST dependency worker | Rust (`git-dep-worker`) | Implemented |
| Secrets management | HashiCorp Vault | Implemented |
| Canvas math engine | Rust → WebAssembly (`@git-viz/wasm-math`) | Implemented |

## 1. Backend — Go

Go's goroutines and M:N scheduler handle thousands of concurrent WebSocket
connections (the Yjs relay) without thread exhaustion. `go-git` manipulates
Git objects natively in memory — no shelling out to OS git binaries and no C
bindings.

Used for: REST endpoints, JWT issuance/validation, GitHub OAuth2 flow,
bare-repository cloning and fetching, topology extraction and layout, the
room-scoped WebSocket relay hub, and schema migration.

## 2. Frontend — TypeScript, React, PixiJS

TypeScript enforces the wire contract with the Go backend: the
`@git-viz/shared-types` package mirrors the Go structs' JSON tags
(snake_case), so contract drift is a compile error rather than a runtime
surprise. React manages the DOM HUD; PixiJS owns the WebGL context.

> **Version note:** PixiJS is pinned to v7 together with `@pixi/react` v7
> because the component layer uses the v7 imperative Graphics API
> (`lineStyle`/`beginFill`) and the v7 `Stage`/`Container`/`Graphics`
> components, which PixiJS v8 removed. The rationale is documented in
> `apps/frontend/vite.config.ts`.

## 3. Graph Workers — Rust

Parsing dependency manifests (`go.mod`, `package.json`) across many
repositories requires deterministic performance without garbage-collection
pauses. The `git-dep-worker` crate (`apps/worker`) parses manifests and
generates cross-repo dependency links, posting them to the backend.

Client-side spline and culling math is implemented in Rust compiled to
WebAssembly (`packages/wasm-math`, the `@git-viz/wasm-math` package). It
exposes viewport culling and Bezier flattening over packed `Float32Array`
buffers, feeding coordinate arrays straight into the WebGL line buffer. The
frontend loads it lazily and falls back to an identical pure-TypeScript
implementation when wasm is unavailable, so behavior is unchanged and only the
performance differs.

## 4. Infrastructure — Podman, Kubernetes, HCL

Local orchestration uses a Compose file compatible with both Podman and
Docker. Cluster deployment uses plain Kubernetes manifests composed with
Kustomize (base + production overlay). Immutable infrastructure definitions
and Vault policies (HCL/Terraform) have scaffolding under `infra/terraform/`
and are part of the roadmap.
