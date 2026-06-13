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
  and repositories; the topology endpoint enforces per-repo team ownership.
- [x] **Dedicated secrets management layer** — the repository-credential
  master key resolves from HashiCorp Vault (fail-closed) via
  `src/secrets.ResolveMasterKey`, falling back to env/dev key only when Vault
  is not configured.
- [x] **GitHub identity persistence** — the OAuth callback fetches the real
  GitHub profile and orgs, upserts a `users` row, maps the primary org to a
  team, and records membership (org admins become Team Owner).

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
- [x] **Viewport culling** — `NodeEngine` renders only nodes and connectors
  inside the padded visible world rectangle (resize-aware).
- [x] **Rust → WebAssembly canvas math engine** — `@git-viz/wasm-math`
  computes viewport culling and Bezier connector flattening over packed
  Float32Array buffers, with an identical pure-TS fallback.

## 3. Navigation & Search

- [x] **Indexed data lookup** — instant client-side filtering over loaded
  nodes by hash, author, and message.
- [x] **Selective visibility retention** — split (origin) and merge commits
  are always retained under filtering, preserving structural context.
- [x] **Multi-repo canvas synchronization** — unified chronological layout
  across repositories with `<RepoID>_<SHA>` collision-free node ids.
- [x] **Inverted-index search datastore** — Meilisearch full-text search
  (`src/search`, `GET /api/v1/search`), indexed on demand, degrading to 503
  when the index is unreachable; the HUD runs an on-submit deep search.
- [x] **Selective branch visibility toggles** — a "tagged commits only"
  filter and per-branch (lane) hiding in the HUD, composed with search and
  the structural retention rule so splits/merges always stay visible.

## 4. Collaborative State Synchronization

- [x] **Cursor tracking** — Yjs awareness broadcasting of world-space
  pointer positions with per-room isolation.
- [x] **Manual dependency line drawing** — drawing mode persists
  `AnnotationVector`s into a shared `Y.Array`, rendered for all room
  participants.
- [x] **Server-side Yjs document persistence** — the relay appends every
  inbound update to a `yjs_updates` log and replays a room's history to a
  lone joining client, so drawn annotations survive disconnects.
- [x] **Event-driven git synchronization** — `POST /api/v1/webhooks/github`
  verifies the HMAC signature and triggers an async repo fetch + reindex.
- [x] **Server-side graph aggregation** — `?max_nodes=N` collapses maximal
  linear commit runs into aggregate nodes (`kind`/`count`), re-running layout;
  the frontend renders them as cluster glyphs.
- [x] **Semantic AST dependency parser worker** — the Rust `git-dep-worker`
  parses `go.mod`/`package.json` and generates cross-repo dependency links,
  ingested via `POST /api/v1/dependency-links` and rendered as dashed
  connectors on the canvas.

## Enhancements beyond the original specification

- [x] **Selective per-author filtering** — a HUD "Authors" popover hides
  commits by author, composed with the other filters and the retention rule.
- [x] **Saved canvas views** — name, save, load, and delete view snapshots
  (viewport + every active filter) persisted per user via `/api/v1/views`.
- [x] **WASM layout pass ("Recompact")** — the engine re-lays-out the visible
  subset client-side so filtering closes branch-lane gaps.

## Remaining

No outstanding features. Possible future directions: streaming/virtualized
topology loading for million-commit repositories, and a presence/permissions
model for shared saved views.
