# API Reference

The backend serves a JSON REST API plus a binary WebSocket endpoint. All
examples assume the default base URL `http://localhost:8080`.

| Endpoint | Method | Auth | Purpose |
| :--- | :--- | :--- | :--- |
| `/health` | GET | none | Liveness probe |
| `/api/v1/auth/login` | GET | none | Development login (single-tenant default identity) |
| `/api/v1/auth/github/login` | GET | none | Start the GitHub OAuth2 flow |
| `/api/v1/auth/github/callback` | GET | OAuth state | Complete the OAuth2 flow, issue a JWT |
| `/api/v1/topology` | GET | Bearer JWT | Unified multi-repo commit topology |
| `/ws/<room>` | WebSocket | none | Yjs CRDT relay room |

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

> GitHub profile → persisted user mapping is a roadmap item; until it lands
> the callback issues the single-tenant default identity.

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

> The relay holds no server-side document state; peers answer each other's
> Yjs sync steps. A client alone in a room therefore starts from an empty
> document. Server-side snapshots are on the roadmap.

## Operations

### `GET /health`

Returns `200 OK` with body `OK`. Suitable for container healthchecks and
load-balancer probes.
