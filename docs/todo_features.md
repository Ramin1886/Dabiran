# Feature Roadmap

Implementation checklist for the platform specification. Checked items are
implemented and covered by the test suites.

## 1. Multi-Tenancy & Authorization

- [x] **RBAC policies** — `Admin`, `Team Owner`, `Team Member` roles carried
  in JWT claims and enforced by `RequireAuth`/`RequireRole` middleware, with
  the role hierarchy persisted in the PostgreSQL schema.
- [x] **OAuth provider authentication** — GitHub OAuth2 authorization-code
  flow with `crypto/rand` CSRF state validation and internal JWT issuance.
- [x] **Team-to-repo 1:N structure** — foreign-key isolation between teams
  and repositories in the schema; single-tenant guard on the topology
  endpoint pending persisted team ownership checks.
- [ ] **Dedicated secrets management layer** (Vault/KMS) — decouple tenant
  cryptographic keys and credentials from the standard persistence layer.
- [ ] **GitHub identity persistence** — map OAuth profiles onto `users`
  rows and an org→team policy (currently the callback issues the
  single-tenant default identity).

## 2. Visualization Engine

- [x] **Chronological left-to-right layout** — Unix-epoch-scaled `x_offset`
  assignment without overlap.
- [x] **Y-axis lane sorting** — oldest-origin lane assignment with primary
  parent lane takeover.
- [x] **Bezier connectors** — cubic curves for lane-crossing splits and
  merges, straight segments within a lane.
- [x] **Contextual click properties** — full-metadata floating panel for the
  selected node.
- [x] **Label priorities** — `tag > short_hash` rendering logic.
- [x] **Infinite viewport** — pointer-anchored zoom and drag panning.
- [ ] **Viewport culling** — restrict draw calls to nodes inside the
  visible window for very large graphs.

## 3. Navigation & Search

- [x] **Indexed data lookup** — instant client-side filtering over loaded
  nodes by hash, author, and message.
- [x] **Selective visibility retention** — split (origin) and merge commits
  are always retained under filtering, preserving structural context.
- [x] **Multi-repo canvas synchronization** — unified chronological layout
  across repositories with `<RepoID>_<SHA>` collision-free node ids.
- [ ] **Inverted-index search datastore** (Elasticsearch/Meilisearch) for
  cross-repository full-text queries at scale.
- [ ] **Selective branch visibility toggles** — "tagged commits only" and
  per-branch hiding built on the retention algorithm.

## 4. Collaborative State Synchronization

- [x] **Cursor tracking** — Yjs awareness broadcasting of world-space
  pointer positions with per-room isolation.
- [x] **Manual dependency line drawing** — drawing mode persists
  `AnnotationVector`s into a shared `Y.Array`, rendered for all room
  participants.
- [ ] **Server-side Yjs document persistence** — snapshot CRDT documents to
  PostgreSQL so annotations survive when all clients disconnect.
- [ ] **Event-driven git synchronization** — webhook ingress replacing
  fetch-on-request refresh.
- [ ] **Server-side graph aggregation** — semantic-zoom clustering of legacy
  linear commit runs to bound client payloads.
- [ ] **Semantic AST dependency parser worker** — auto-generate cross-repo
  visual links from manifests (`package.json`, `go.mod`).
