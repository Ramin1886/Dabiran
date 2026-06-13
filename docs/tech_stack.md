# Technology Stack Matrix

Based on the architectural requirements for high concurrency, real-time binary synchronization, and massive graph visualization, the optimal language stack prioritizes memory safety, highly efficient context switching, and strict typing.

## 1. Core API, Git Ingestion & WebSocket Relay (Backend)
**Language of Choice:** Go (Golang)
*   **Rationale:** Go's lightweight goroutines and M:N scheduler are built specifically for handling thousands of concurrent WebSocket connections (the Yjs CRDT synchronization pipeline) without thread exhaustion. Utilizing `go-git` natively allows the backend to manipulate Git objects entirely in memory without shelling out to OS-level Git binaries or relying on slow C-bindings.
*   **Use Cases:** REST API definitions, OAuth2 token rotation, Webhook ingress, and the primary WebSocket Hub.

## 2. Semantic AST Parsers & Graph Aggregation Workers
**Language of Choice:** Rust
*   **Rationale:** Parsing Abstract Syntax Trees (`go.mod`, `package.json`, etc.) across thousands of enterprise repositories and dynamically aggregating massive Git Directed Acyclic Graphs (DAGs) requires predictable, deterministic performance. Rust provides C-level speed and zero-cost abstractions with absolute memory safety, eliminating Garbage Collection (GC) pauses that would otherwise degrade worker throughput during heavy computational graph traversals.

## 3. Frontend WebGL Canvas & Client Application Logic
**Language of Choice:** TypeScript (with WebAssembly/Rust for Math)
*   **Rationale:** TypeScript is mandatory for enforcing strict data contracts between the Golang backend payloads and the frontend client. The React framework manages the DOM UI, while PixiJS controls the WebGL context.
*   **Performance Vector:** For calculating complex Bezier splines and executing mathematical bounds culling on tens of thousands of coordinate nodes, the mathematical engine should ideally be written in **Rust and compiled to WebAssembly (WASM)**. This bypasses the V8 JavaScript engine's JIT compilation overhead, feeding binary coordinate arrays directly into the WebGL buffer.

## 4. Infrastructure & Secrets Provisioning
**Language of Choice:** HCL (HashiCorp Configuration Language) & Go
*   **Rationale:** HCL is required for defining immutable infrastructure topologies (HashiCorp Vault policies, Podman deployment logic). Any custom infrastructure orchestration operators bridging Podman, PostgreSQL, and Vault should be written in Go, as it is the native language of the cloud-native ecosystem.
