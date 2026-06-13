# PROGRESS

Build-out status for the Collaborative Git Visualization Platform. The
initial scaffold did not compile or run; it is now a fully building,
tested, and live-verified stack with the feature roadmap essentially
cleared. Verification gates (all green):

- Backend: `go build/vet/test ./...` — 10 packages, tested against live PostgreSQL.
- Worker: `cargo build/test` — 15 tests.
- Frontend + packages: `npm test` — 72 frontend + 7 shared-types + utils.
- Infra: compose parses; `kustomize build` of base and production overlay.

## Implemented

### Foundations
- npm workspaces; `@git-viz/shared-types` (wire contracts mirroring the Go
  JSON tags) and `@git-viz/utils` (hash codec, viewport math).
- Backend compiles and runs; PostgreSQL optional at boot; idempotent schema
  migration plus single-tenant identity seed so the dev login works against a
  fresh database.

### Identity & access
- RBAC JWT claims + `RequireAuth`/`RequireRole` middleware.
- GitHub OAuth2 with CSRF state; the callback fetches the real profile/orgs,
  upserts a user, maps the primary org to a team, and records membership
  (org admins → Team Owner).
- Repository management API: register repos with credentials encrypted at
  rest; topology enforces per-repo team ownership.

### Visualization & search
- Chronological layout, branch lanes, Bezier connectors, tag/short-hash
  labels, commit inspection panel.
- Infinite viewport with pointer-anchored zoom/pan and **viewport culling**.
- **Server-side graph aggregation** (`?max_nodes`) collapsing linear runs
  into cluster nodes, rendered as distinct glyphs.
- **Meilisearch** full-text search (indexed on demand, graceful 503), with an
  on-submit deep-search HUD affordance over the instant client-side filter.

### Collaboration & automation
- Yjs CRDT relay with live cursors and drawn annotation vectors.
- **Server-side Yjs persistence**: append-only update log replayed to lone
  joiners, so annotations survive disconnects.
- **Webhook ingress** (`/api/v1/webhooks/github`) with HMAC verification
  driving async fetch + reindex.
- **Rust `git-dep-worker`**: parses `go.mod`/`package.json`, generates
  cross-repo dependency links, ingested via `/api/v1/dependency-links` and
  rendered as dashed connectors on the canvas.

### Secrets & operations
- **HashiCorp Vault** sources the repo-credential master key (fail-closed),
  falling back to env/dev key only when Vault is not configured.
- Compose: Postgres + Meilisearch + Vault (dev) + on-demand worker, with
  healthchecks and fully-qualified images for rootless Podman.
- **Real CD pipeline** (`cicd/deploy.yaml`): builds and pushes backend +
  frontend images to GHCR on release/manual dispatch, then deploys via
  Kustomize when a `KUBE_CONFIG` secret is present. `cicd/build.yaml` runs
  the CI test/validation gate.

## Skipped / deferred (and why)

- **Selective branch-visibility toggles** — UI work on top of the existing
  retention algorithm; not yet built. The only open roadmap item.
- **Rust → WASM canvas math engine** — a performance optimization; the
  current TypeScript math is adequate at present scale.
- **GitHub profile fetch in tests** — exercised via a fake `GitHubClient`;
  the real network call is integration-only.
- **`packages/ui-components`** — no shared cross-app DOM components exist yet;
  an empty library would be speculative.

## Operating notes

- Local dev: `cd infra && podman-compose up -d postgres` (or add
  `meilisearch`, `vault`). The backend degrades gracefully when any optional
  service is down. Run the worker on demand:
  `podman-compose run --rm worker --root /repos --api http://backend:8080 --token <jwt>`.
- Secrets: set `JWT_SECRET`, `REPO_CRED_KEY` (or Vault), `GITHUB_*`, and
  `GITHUB_WEBHOOK_SECRET` in production; templates in `apps/backend/.env.example`.
- CD: publishing requires no setup (GHCR via the built-in token); deploying
  requires the `KUBE_CONFIG` repository secret and replacing `OWNER` in the
  production overlay.

## Next steps

1. Selective branch-visibility toggles in the HUD.
2. Periodic compaction of the `yjs_updates` log into snapshots.
3. Schedule the dependency worker (cron/webhook) instead of manual runs.
4. Rust → WASM math engine for very large graphs.
