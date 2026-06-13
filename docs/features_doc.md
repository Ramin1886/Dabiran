# Feature Specification

Functional specification of the platform. Items marked *(roadmap)* are
planned and tracked in [todo_features.md](./todo_features.md); everything
else is implemented.

## 1. Multi-Tenancy & Authorization

Role-based access control isolates tenants and prevents metadata leaks
across teams.

| Role | Capabilities |
| :--- | :--- |
| Admin | Provisions teams and assigns users |
| Team Owner | Registers repositories and authentication credentials for the team |
| Team Member | Read access to team repositories; creates collaborative annotations |

* Sessions are JWTs (HS256) carrying `user_id`, `team_id`, and `role`
  claims; middleware enforces authentication on protected routes and role
  checks where required.
* The database schema enforces team-to-repository 1:N isolation through
  foreign keys.
* GitHub OAuth2 sign-in is implemented with CSRF state validation. Identity
  mapping from GitHub profiles onto persisted users/teams is *(roadmap)*;
  until then the backend issues a single-tenant default identity.
* Repository credentials are encrypted with AES-256-GCM before storage. A
  dedicated secrets layer (Vault/KMS) for master keys is *(roadmap)*.

## 2. Navigation, Filtering & Multi-Repo Canvas

* **Commit search** — the HUD search field filters the rendered graph
  instantly by hash, author, or message (case-insensitive, client-side).
* **Structural retention rule** — filtering always retains split commits
  (more than one child) and merge commits (more than one parent), so the
  skeleton of the topology stays legible regardless of the filter.
* **Label priority** — nodes are labeled with their tag when one exists,
  otherwise with their short hash.
* **Unified multi-repo canvas** — `GET /api/v1/topology?repo_ids=1,2,…`
  merges several repositories onto a single chronological canvas; node ids
  are prefixed `<RepoID>_<SHA>` to prevent collisions.
* **Inverted-index search service** (Elasticsearch/Meilisearch) for
  cross-repository full-text queries over millions of commits *(roadmap)*.
* **Selective visibility toggles** — "tagged commits only", per-branch (lane)
  hiding, and per-author hiding in the HUD. All compose with search and honor
  the structural retention rule, so split and merge commits always stay
  visible and an isolated branch keeps its origin/merge bounds.
* **Layout recompaction** — a "Recompact" toggle re-lays-out the visible
  commits client-side (via the WebAssembly engine) so filtering closes
  branch-lane gaps.
* **Saved canvas views** — name, save, load, and delete snapshots of the
  viewport + all active filters, persisted per user (`/api/v1/views`).
* **Automated dependency resolution** — an AST/manifest parser worker that
  auto-links related commits across repositories *(roadmap)*.

## 3. Real-Time Collaboration Canvas

Miro-style collaboration layered on the parsed Git topology:

* **Infinite viewport** — pointer-anchored wheel zoom and drag panning.
* **Live cursors** — every collaborator's pointer is broadcast through the
  Yjs awareness protocol and rendered at world coordinates, so cursors stay
  anchored to the graph under any zoom level.
* **Manual dependency drawing** — in drawing mode, dragging creates a vector
  between arbitrary canvas points (e.g. linking commits across
  repositories). Completed vectors are appended to a shared CRDT array and
  appear for all room participants; they persist for the lifetime of the
  collaborative session.
* **Commit inspection** — clicking a node opens a floating panel with the
  full message, author, date, tag, and parent lineage hashes.
* **Conflict-free state** — all shared state is CRDT-backed (Yjs); there are
  no locks and concurrent edits converge deterministically.
* **Event-driven repository sync** via webhooks *(roadmap — refresh
  currently happens on fetch)*.
* **Server-side graph aggregation** for very large DAGs *(roadmap)*.
