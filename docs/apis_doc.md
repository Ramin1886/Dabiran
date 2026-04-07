# Application Programming Interfaces (API)

This document precisely outlines the RESTful boundaries and Binary WebSocket integration schemas between the presentation layer (`apps/frontend/`) and the core git abstraction engine (`apps/backend/`).

## Authentication Protocol

The platform implements standard OAuth2 / JWT bearer token mechanics scoped rigidly by the Role-Based Access Control (RBAC) definitions.

### `POST /api/v1/auth/login`

Validates tenant identity and issues a scoped access token.

| Component | Specification |
| :--- | :--- |
| **Description** | Interacts exclusively with internal `Users` PostgreSQL records or delegates to SSO providers. |
| **Request Payload** | `{"email": "string", "provider_token": "string"}` |
| **Response Payload** | `{"access_token": "jwt_string", "role": "admin" \| "member"}` |
| **Security Risk** | Tokens must be transferred securely over TLS. Database never stores plaintext passwords. |

---

## Repository Management

Handles the CRUD lifecycle of bare Git repositories stored natively within the infrastructure host cache.

### `GET /api/v1/repos`

Fetches active repositories available for the requesting tenant.

| Component | Specification |
| :--- | :--- |
| **Description** | Scopes visibility based on `team_id` derived automatically from the JWT bearer token. |
| **Query Params** | `?team_id=integer` |
| **Response Payload** | `[ {"id": integer, "name": "string", "url": "string"} ]` |

### `POST /api/v1/repos`

Registers a new remote repository and triggers an immediate asynchronous bare-clone synchronization sequence.

| Component | Specification |
| :--- | :--- |
| **Description** | Raw `auth_secret` fields are captured and subjected to strict AES-256-GCM encryption before persistence to the `repositories` DB schema. |
| **Request Payload** | `{"team_id": integer, "name": "string", "url": "string", "auth_type": "ssh" \| "https", "auth_secret": "string"}` |
| **Response Payload** | `{"id": integer, "status": "cloning_initiated"}` |

---

## Git Topology Visualization

The core engine interacting with the `go-git` abstract logic.

### `GET /api/v1/topology`

Extracts and sequences temporal branch topology based on chronological creation. Optimized specifically for handling high-volume DOM layout mathematics.

| Component | Specification |
| :--- | :--- |
| **Description** | Traverses internal `.git/objects` and outputs absolute ancestor lineage sorted from oldest Unix Epoch origin. |
| **Query Params** | `?repo_id=integer` <br> `?limit=integer` (Default: 500) <br> `?offset=integer` <br> `?branch_filter=string` <br> `?tags_only=boolean` |
| **Response Payload** | Array of structural node constraints (See Diagram Below). |

#### Topology Array Output Scheme

```json
[
  {
    "hash": "a1b2c3d4e5f6g7h8",
    "short_hash": "a1b2c3d",
    "author": "Alice",
    "message": "Initial architectural commit",
    "date": "RFC3339 String",
    "parents": [ "previous_hash_string" ] 
  }
]
```

> [!IMPORTANT]
> The `parents` array natively dictates the WebGL `<NodeEngine />` coordinate diagonals. A node returning multiple parent hashes instantly triggers merge-curve rendering logic across Y-Axis lanes.

---

## Real-Time Collaboration Canvas

### `GET /ws` [Upgrade: websocket]

Binary endpoint relaying raw `Uint8Array` structs matching the Yjs CRDT synchronization protocol.

| Component | Specification |
| :--- | :--- |
| **Description** | Binds to Gorilla Mux `Upgrader`. Broadcasts conflict-free state mutations across all active clients monitoring a specific map. |
| **Connection Params** | Upgrade requests must pass `?room_id=repo_{id}` to multiplex clients logically. |
| **Limitations** | Payload maximums are isolated strictly to 10MB bounds mimicking Yjs vector mapping to shield internal buffer memory fault overlaps. |
| **Implementation** | The Go hub operates as an unintelligent relay; all CRDT reconciliation logic executes exclusively in the frontend React client instances. |
