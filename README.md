# Collaborative Git Visualization Platform

A self-hosted, cloud-native application that transforms Git repository
histories into an interactive, real-time collaborative WebGL canvas. Instead
of reading linear CLI logs, teams explore branch topologies on an infinite,
Miro-style workspace: commits are laid out chronologically across branch
lanes, splits and merges are drawn as Bezier connectors, and collaborators
share live cursors and hand-drawn dependency annotations synchronized through
conflict-free CRDTs.

The platform targets zero-trust, self-hosted environments: repositories are
mirrored into local bare clones, credentials are encrypted at rest
(AES-256-GCM), and all components run as rootless containers.

## Key Capabilities

| Capability | Description |
| :--- | :--- |
| Topology canvas | Chronological left-to-right commit layout with branch lanes and Bezier split/merge connectors, rendered in WebGL (PixiJS) |
| Infinite viewport | Pointer-anchored wheel zoom and drag panning over arbitrarily large graphs |
| Multi-repo view | Multiple repositories unified onto one canvas with collision-free `<RepoID>_<SHA>` node identifiers |
| Real-time collaboration | Yjs CRDT synchronization over WebSockets: live cursors and persistent drawn annotation vectors |
| Search & filtering | Instant client-side filtering by hash, author, or message that always retains split and merge nodes for structural context |
| Authentication | JWT sessions with RBAC claims; GitHub OAuth2 flow with CSRF protection |
| Persistence | PostgreSQL schema for users, teams, repositories, and annotations with 1:N team-to-repository isolation |

## Documentation

| Document | Audience | Content |
| :--- | :--- | :--- |
| [Architecture](./docs/architecture.md) | Engineers | System design, collaboration pipeline, rendering model |
| [Technology Stack](./docs/tech_stack.md) | Engineers | Language and framework choices with rationale |
| [Local Setup](./docs/local-setup.md) | Engineers | Step-by-step local development environment |
| [Features](./docs/features_doc.md) | All | Functional specification of the platform |
| [API Reference](./docs/apis_doc.md) | Engineers | REST and WebSocket endpoint contracts |
| [Function Reference](./docs/functions_doc.md) | Engineers | Per-package function documentation |
| [Roadmap](./docs/todo_features.md) | All | Implemented and planned feature checklist |
| [Cloud Administrator Manual](./docs/manuals/cloud-admin-guide.md) | Operators | Hosting, deployment, secrets, backups, troubleshooting |
| [Access Administration Manual](./docs/manuals/access-admin-guide.md) | Administrators | Roles, users, teams, and repository access provisioning |
| [User Manual](./docs/manuals/user-guide.md) | Team owners & members | Using the collaborative canvas |

## Monorepo Layout

| Path | Purpose |
| :--- | :--- |
| `apps/frontend/` | React + PixiJS WebGL canvas application (TypeScript, Vite) |
| `apps/backend/` | Go API server: bare-git engine, REST API, Yjs WebSocket relay |
| `apps/worker/` | Rust `git-dep-worker`: parses manifests, generates cross-repo dependency links |
| `packages/shared-types/` | Cross-app TypeScript data contracts mirroring the Go wire format |
| `packages/utils/` | Shared pure helpers (hash codec, viewport math) |
| `packages/wasm-math/` | Rust→WebAssembly canvas math engine (viewport culling, Bezier flattening) |
| `infra/` | Podman/Docker Compose, Kubernetes manifests (Kustomize), Terraform scaffolding |
| `cicd/` | Build/test and deployment pipeline definitions |
| `docs/` | Architecture, guides, API reference, and manuals |
