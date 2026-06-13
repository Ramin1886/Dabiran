# API Reference

The backend serves a JSON REST API plus a binary WebSocket endpoint. All
examples assume the default base URL `http://localhost:8080`.

| Endpoint | Method | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `/health` | GET | none | Liveness probe |
| `/api/v1/auth/login` | GET | none | Development login (single-tenant default identity) |
| `/api/v1/auth/github/login` | GET | none | Start the GitHub OAuth2 flow |
| `/api/v1/auth/github/callback` | GET | OAuth state | Complete the OAuth2 flow, issue a JWT |
| `/api/v1/topology` | GET | Bearer JWT | Unified multi-repo commit topology (supports `max_nodes` aggregation) |
| `/api/v1/repositories` | POST/GET | Bearer JWT | Register (Team Owner) / list team repositories |
| `/api/v1/search` | GET | Bearer JWT | Meilisearch full-text commit search |
| `/api/v1/dependency-links` | POST/GET | Bearer JWT | Ingest / list cross-repo dependency links |
| `/api/v1/views` | POST/GET | Bearer JWT | Save / list per-user canvas views |
| `/api/v1/views/{id}` | DELETE | Bearer JWT | Delete an owned canvas view |
| `/api/v1/webhooks/github` | POST | HMAC signature | GitHub push webhook (event-driven sync) |
| `/ws/<room>` | WebSocket | none | Yjs CRDT relay room (server-side persisted) |

## Authentication

Sessions are JWTs (HS256, 24-hour expiry) carrying `user_id`, `team_id`, and
`role` claims. Protected endpoints expect the header
`Authorization: Bearer <token>`.

### `GET /api/v1/auth/login`

Development login. Issues a JWT for the single-tenant default identity
(`Team Owner`, team `100`). Intended for local development and evaluation;
disable or replace behind your proxy in hardened deployments.

**Response `200`**

```json
{ "access_token": "<jwt>", "role": "Team Owner" }
```

### `GET /api/v1/auth/github/login`

Starts the GitHub OAuth2 authorization-code flow.

| Aspect | Behavior |
| :--- | :--- |
| CSRF protection | Generates a 32-byte `crypto/rand` state, stored in a 10-minute `HttpOnly`, `SameSite=Lax` cookie |
| Response | `307 Temporary Redirect` to GitHub's authorize URL carrying the same state |
| Configuration | `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `OAUTH_REDIRECT_URL` environment variables |
| Scopes requested | `read:org`, `repo` |

### `GET /api/v1/auth/github/callback`

Completes the flow. Must be registered as the OAuth app's authorization
callback URL.

| Aspect | Behavior |
| :--- | :--- |
| Query params | `code`, `state` (set by GitHub) |
| State validation | `state` must match the login cookie; the cookie is single-use and expired immediately |
| Token exchange | Authorization code exchanged server-side for a GitHub token |
| Errors | `401 Unauthorized` on state mismatch, failed exchange, or invalid token |

**Response `200`**

```json
{ "access_token": "<jwt>", "role": "Team Owner" }
```

## Topology

### `GET /api/v1/topology`

Extracts and chronologically lays out the commit graph of one or more
repositories as a single unified node array.

| Aspect | Specification |
| :--- | :--- |
| Auth | `Authorization: Bearer <jwt>` required |
| Query params | `repo_ids` — comma-separated repository ids, e.g. `repo_ids=1,2` |
| Repository resolution | Each id is resolved against the `repositories` table (clone/fetch by URL) first, then against local bare repos `repos/mock_<id>.git`, then `repos/repo_<id>.git` |
| Hash prefixing | `hash` and every entry of `parents` are prefixed `<RepoID>_<SHA>` so multi-repo graphs never collide |
| Layout | Nodes sorted by author date; `x_offset` is seconds-from-oldest × 0.05 px; `lane` is the branch track index |
| Aggregation | Optional `max_nodes=<N>`: when the extracted count exceeds N, maximal runs of linear commits collapse into `kind:"aggregate"` nodes carrying `count`, and the layout re-runs |

**Response `200` — `CommitNode[]`** (contract mirrored by
`@git-viz/shared-types`)

```json
[
  {
    "hash": "1_a1b2c3d4e5f6a7b8…",
    "short_hash": "a1b2c3d",
    "author": "Alice",
    "message": "Initial architectural commit",
    "date": "2026-01-01T00:00:00Z",
    "parents": ["1_<parent-sha>"],
    "lane": 0,
    "x_offset": 0,
    "repo_id": "1",
    "tag": "v1.0.0"
  }
]
```

**Errors**

| Status | Condition |
| :--- | :--- |
| `400 Bad Request` | `repo_ids` missing or empty |
| `401 Unauthorized` | Missing/malformed/invalid/expired JWT |
| `403 Forbidden` | Token's team is not authorized for the repositories |
| `404 Not Found` | None of the requested ids resolved to a repository |

## Repository Management

### `POST /api/v1/repositories`

Registers a repository for the caller's team. **Team Owner** role required.
The `auth_secret` (PAT or SSH key) is encrypted with AES-256-GCM
(`crypto.Encrypt`) using the master key resolved from Vault or env before
storage and is **never** returned by any endpoint.

**Body** `{"name", "url", "auth_type", "auth_secret"}` — `auth_type` is one of
`"https"`, `"ssh"`, or `""` (anonymous).

**Response `201`** `{"id", "name", "url"}` (no credential).

### `GET /api/v1/repositories`

Lists the caller team's repositories as `[{"id", "name", "url"}]`. Any
authenticated user.

## Search

### `GET /api/v1/search`

Meilisearch-backed full-text commit search across the team's repositories.

| Aspect | Specification |
| :--- | :--- |
| Auth | Bearer JWT; same per-repo team authorization as topology |
| Query params | `q` (text), `repo_ids` (comma-separated) |
| Response `200` | `{"hits": [{"hash","short_hash","author","message","repo_id","tag"}]}` |
| `503` | The search index is unreachable (clients fall back to client-side filtering) |

Commits are indexed on demand after a successful topology extraction and on
webhook push events.

## Dependency Links

### `POST /api/v1/dependency-links`

Ingests auto-generated cross-repository dependency links (produced by the
Rust `git-dep-worker`). Stored as `dependency`-type annotation rows. The
`from_repo` of every link must belong to the caller's team (else `403`).

**Body** `{"links": [{"from_repo","to_repo","via","kind"}]}` —
`kind` is `"go"` or `"npm"`, `via` is the linking module/package.
**Response `200`** `{"stored": <n>}`.

### `GET /api/v1/dependency-links`

Returns the stored links whose `from_repo` is in `repo_ids` (team-scoped) as
`{"links": [...]}`.

## Saved Canvas Views

Per-user named snapshots of the frontend view (viewport + active filters).
The `state` field is an opaque JSON string the frontend owns; the backend
stores and returns it verbatim.

### `POST /api/v1/views`

Body `{"name", "state"}` (both required, `400` if empty). Stored against the
caller's `user_id`/`team_id`. Response `201` `{"id","name","state"}`.

### `GET /api/v1/views`

Returns `{"views": [{"id","name","state"}...]}` for the caller only, newest
first.

### `DELETE /api/v1/views/{id}`

Deletes a view owned by the caller (`204`); returns `404` for a missing or
non-owned id (leaking nothing about other users). All three require a JWT and
return `503` when no database is attached.

## Webhooks

### `POST /api/v1/webhooks/github`

GitHub push webhook for event-driven repository sync. The raw body is
verified against `X-Hub-Signature-256` (HMAC-SHA256 over
`GITHUB_WEBHOOK_SECRET`; `401` on mismatch — verification is skipped with a
logged warning in dev when the secret is unset). On `push` events the matching
repository is fetched asynchronously and re-indexed (`202`); other events
return `204`. Authenticated by the signature, not a JWT.

## Real-Time Synchronization

### `GET /ws/<room>` (WebSocket upgrade)

Connects the client into a CRDT relay room. Compatible with the
`y-websocket` provider, which appends the room name as a path segment:

```ts
new WebsocketProvider('ws://host:8080/ws', 'repo_map_1', ydoc)
// connects to ws://host:8080/ws/repo_map_1
```

| Aspect | Specification |
| :--- | :--- |
| Room resolution | Path segment after `/ws/`; falls back to the `?room_id=` query parameter; defaults to `default` |
| Room convention | One room per repository map: `repo_map_<id>` |
| Semantics | Binary frames are relayed verbatim to every other client in the same room (sender excluded); rooms are fully isolated |
| Payloads | Yjs document updates (shared `annotations` array) and awareness frames (live cursors) |
| Limits | 1 MiB read limit per frame; ping/pong liveness; slow consumers are disconnected rather than blocking the room |

> The relay persists every inbound update to an append-only `yjs_updates`
> log and replays a room's history to a lone joining client, so drawn
> annotations survive when all clients disconnect (no-op when no database is
> attached). Peers still answer each other's live Yjs sync steps.

## Operations

### `GET /health`

Returns `200 OK` with body `OK`. Suitable for container healthchecks and
load-balancer probes.
