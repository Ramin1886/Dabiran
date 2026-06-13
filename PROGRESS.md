# PROGRESS

Status report per phase for the team build-out of the Collaborative Git
Visualization Platform. Verification gate at the end of each phase:
`go build/vet/test ./...` (backend) and `npm run build -w frontend` +
`npm test` (workspaces) — all green at time of writing.

## Phase 0 — Understanding

Read every file under `docs/`, `apps/`, `packages/`, `infra/`, `cicd/`.
Finding: the repo was a documented skeleton that **did not compile or run on
either side** — missing `ws` package, missing Go deps, import paths not
matching the `src/` layout, a go-git tag-API compile error, a stub
`main.tsx` with no Vite scaffold, a broken TS↔Go wire contract
(`xOffset` vs `x_offset`), empty `packages/`, zero tests, and Dockerfiles
building wrong paths.

## Phase 1 — Packages (completed)

- npm workspaces root (`package.json`), `.gitignore`.
- `@git-viz/shared-types`: `CommitNode` (snake_case wire format matching the
  Go JSON tags and `docs/apis_doc.md`), `Role`, `AuthResponse`,
  `CursorState`, `AnnotationVector`, `repoMapRoom()`.
- `@git-viz/utils`: `<RepoID>_<SHA>` prefixed-hash codec, `screenToWorld`,
  pointer-anchored `zoomAt` viewport math.
- 10 vitest tests.

## Phase 2 — Backend (completed)

- New `src/ws`: room-scoped binary relay hub for y-websocket clients
  (path-segment room + `?room_id=` fallback per docs; per-client write pump,
  read limits, ping/pong). Documented limitation: dumb relay — peers answer
  Yjs sync steps; no server-side doc persistence yet.
- New `src/db`: `Connect` + idempotent `Migrate` (users, teams,
  repositories, annotations; 1:N tenant FKs). DB optional at boot —
  warns and continues per local-setup flow.
- Registered documented OAuth routes `/api/v1/auth/github/{login,callback}`
  with crypto/rand CSRF state cookie and `role` in the callback payload.
- Fixed: module import paths (`<module>/src/...`), missing `jwt`/`oauth2`
  deps, tag resolution (lightweight + annotated), `JWT_SECRET` from env,
  topology repo lookup via DB row with filesystem fallback, Dockerfile
  build path. Added `.env.example`.
- 12 `_test.go` files, 40 tests (clean under `-race`).

## Phase 3 — Frontend (completed)

- Full scaffold: `index.html`, `main.tsx` (createRoot), `vite.config.ts`
  (port 3000), strict tsconfig. `pixi.js` pinned to ^7.4 with
  `@pixi/react` ^7.1 because the component layer uses v7 APIs that v8
  removed (rationale documented in `vite.config.ts`).
- Wire contract adopted from `@git-viz/shared-types` (`x_offset`/`lane`
  fix in `NodeEngine`).
- Infinite canvas: pointer-anchored wheel zoom + drag pan; world-space
  cursor broadcasts and drawing vectors.
- Boot pipeline: login → `fetchTopology(['1'])` → CRDT room
  `repo_map_1`; HUD status line, search input (split/merge retention
  filter), drawing-mode toggle, label priority tag > short_hash.
- CRDT fixes: correct y-websocket room (path segment), persisted
  `AnnotationVector`s in a shared `Y.Array('annotations')`.
- Root-context Dockerfile for workspace builds. 8 test files, 41 tests.

## Phase 4 — Infra & CI/CD (completed)

- Compose: postgres healthcheck + healthy-ordering, backend secrets env
  with dev fallbacks (`JWT_SECRET`, `GITHUB_*`, `OAUTH_REDIRECT_URL`),
  bare-repo cache volume, frontend built from repo-root context.
- k8s: `git-viz-secrets` Secret (dev placeholder; production must come
  from Vault/KMS per `docs/architecture.md`), backend deployment wired to
  it via `secretKeyRef`.
- CI (`cicd/build.yaml`): Go vet/test/build, npm workspace test + frontend
  build, compose + kustomize validation. Kustomize verified locally.

## Phase 5 — QA (completed)

- Independent re-runs of all gates: backend 40 tests, frontend 41 tests,
  packages 10 tests — green.
- End-to-end smoke: booted the compiled server against a real bare clone
  (`repos/mock_1.git`); verified `/health`, JWT login, topology JSON
  (15 nodes, all contract fields, chronological `x_offset`,
  `<RepoID>_` prefixes), `401` without a token, and graceful start
  without PostgreSQL.

## Skipped (and why)

- **Vault/KMS secrets layer, Elasticsearch/Meilisearch inverted index,
  webhook-driven sync, server-side graph aggregation, Rust AST dependency
  parser worker, Rust/WASM math engine** — explicitly open roadmap items in
  `docs/todo_features.md` requiring external services/credentials; left
  unchecked there. Env stubs exist where relevant (`.env.example`).
- **GitHub profile fetch in the OAuth callback** — needs real client
  credentials and user persistence + org→team mapping; documented TODO,
  callback still issues the internal JWT per the docs schema.
- **`packages/ui-components`** — no shared DOM components exist yet across
  apps; creating an empty library would be speculative.
- **Test files for YAML manifests** — validated by the CI
  `validate_infra` job (compose config + kustomize build) instead of unit
  tests, which don't apply to declarative manifests.
- **`cicd/deploy.yaml`** — remains a stub: deployment requires registry and
  cluster credentials that are intentionally not in the repo.

## Next steps

1. Persist Yjs documents server-side (snapshot to PostgreSQL per
   `docs/architecture.md` "Async Snapshot") so annotations survive when all
   clients disconnect.
2. Real GitHub identity in the OAuth callback + user/team persistence to
   replace the single-tenant default claims.
3. Repository management API (register repo URL + encrypted credential via
   `crypto.Encrypt`) so topology no longer depends on pre-seeded bare repos.
4. Viewport culling in `NodeEngine` (render only nodes inside the visible
   window) ahead of large-DAG payloads; then server-side aggregation.
5. Webhook ingress for event-driven sync (roadmap §4.1).
