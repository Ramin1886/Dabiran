# Function Reference

Public functions per package. Signatures are abbreviated; see the source for
full GoDoc/JSDoc.

## Backend — Go

### `src/gitengine`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `NewGitEngine` | `storagePath string` | `*GitEngine` | Creates the engine and ensures the bare-repo cache directory exists |
| `(*GitEngine) EnsureRepository` | `ctx, repoID int, url, authType, authSecret` | `*git.Repository, error` | Bare-clones the repository on first use, otherwise opens and force-fetches all refs; supports HTTPS token and SSH key auth |
| `ExtractUnifiedTopology` | `map[string]*git.Repository` | `[]CommitNode, error` | Walks all commits of all repositories, resolves lightweight and annotated tags, prefixes ids with `<RepoID>_`, sorts chronologically, and assigns `x_offset` and branch `lane` |

### `src/api`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `NewAPIServer` | `*GitEngine, *pgxpool.Pool` | `*APIServer` | Binds the git engine and optional DB pool to the HTTP handlers |
| `(*APIServer) LoginMock` | HTTP | JSON | Issues the development JWT (`/api/v1/auth/login`) |
| `(*APIServer) ServeTopology` | HTTP | JSON | JWT-protected unified topology endpoint (`/api/v1/topology`) |
| `(*APIServer) AddRoutes` | `*http.ServeMux` | — | Registers all REST routes including the OAuth pair |
| `RequireAuth` | `http.HandlerFunc` | `http.HandlerFunc` | Middleware: validates the Bearer JWT and injects claims into the request context |
| `RequireRole` | `role string, next` | `http.HandlerFunc` | Middleware: `RequireAuth` plus an exact role check (`403` on mismatch) |

### `src/auth`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `GenerateToken` | `userID, teamID int, role string` | `string, error` | Signs an HS256 JWT with 24-hour expiry; key from `JWT_SECRET` |
| `ValidateToken` | `tokenString string` | `*Claims, error` | Parses and verifies a JWT (HS256 only), returning the claims |
| `GetOAuthConfig` | — | `*oauth2.Config` | Builds the GitHub OAuth2 config from environment variables |
| `HandleLogin` | HTTP | redirect | Starts the OAuth2 flow with a `crypto/rand` CSRF state cookie |
| `HandleCallback` | HTTP | JSON | Verifies state, exchanges the code, and issues the internal JWT |

### `src/crypto`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `Encrypt` | `plaintext, key []byte` (32-byte key) | `string, error` | AES-256-GCM encryption; returns base64(nonce‖ciphertext) |
| `Decrypt` | `encString string, key []byte` | `[]byte, error` | Reverses `Encrypt`, authenticating the ciphertext |

### `src/db`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `Connect` | `ctx, url string` | `*pgxpool.Pool, error` | Opens a pgx pool and verifies liveness with a 5-second ping |
| `Migrate` | `ctx, *pgxpool.Pool` | `error` | Applies the idempotent `CREATE TABLE IF NOT EXISTS` schema (users, teams, repositories, annotations) |

### `src/ws`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `NewHub` | — | `*Hub` | Allocates the room-scoped relay hub |
| `(*Hub) Run` | — | — | Event loop owning all room membership and broadcasts (run as a goroutine) |
| `ServeWs` | `*Hub, http.ResponseWriter, *http.Request` | — | Upgrades the connection, resolves the room (path segment or `?room_id=`), and starts the client's read/write pumps |

## Frontend — TypeScript

### `src/api/client.ts`

| Function | Inputs | Outputs | Description |
| :--- | :--- | :--- | :--- |
| `login` | — | `Promise<AuthResponse>` | Calls `/api/v1/auth/login` on `VITE_API_URL` |
| `fetchTopology` | `repoIds: string[], token: string` | `Promise<CommitNode[]>` | Fetches `/api/v1/topology` with the Bearer header |

### `src/store/useStore.ts` (Zustand)

| Member | Description |
| :--- | :--- |
| `setNodes(nodes)` | Replaces the graph; re-applies any active search filter |
| `setSearchQuery(query)` | Filters `visibleNodes` by hash/author/message while always retaining split and merge commits |
| `setViewportTransform(t)` | Stores the world container `{x, y, scale}` |
| `setSelectedNode(hash)` / `setDrawingState(b)` | Commit panel selection / drawing-mode toggle |

### `src/store/useCRDT.ts` (Zustand + Yjs)

| Member | Description |
| :--- | :--- |
| `connect(room)` | Creates a fresh `Y.Doc`, connects a `WebsocketProvider` to `/ws`, subscribes to awareness cursors and the shared `annotations` array |
| `updateCursor(x, y)` | Broadcasts the local pointer (world coordinates) via awareness |
| `addAnnotation(vector)` | Appends an `AnnotationVector` to the shared `Y.Array` |

### Components

| Component | Description |
| :--- | :--- |
| `App` | Boot pipeline (login → topology → store), CRDT room join, HUD (search, draw toggle, status line) |
| `InteractiveCanvas` | Infinite viewport: anchored wheel zoom, drag pan, drawing mode, remote cursors, persisted annotations |
| `NodeEngine` | Renders visible commits and Bezier parent connectors; label priority tag > short hash |
| `CommitPanel` | Floating glassmorphism panel with full metadata of the selected commit |
| `hexToNumber(hex)` | CSS hex color → PixiJS numeric color |

## Shared Packages

### `@git-viz/shared-types`

| Export | Description |
| :--- | :--- |
| `CommitNode`, `Role`, `AuthResponse`, `CursorState`, `AnnotationVector` | Wire contracts mirroring the Go JSON tags (snake_case) |
| `repoMapRoom(repoId)` | Canonical CRDT room id, `repo_map_<id>` |

### `@git-viz/utils`

| Export | Description |
| :--- | :--- |
| `makePrefixedHash(repoId, sha)` / `parsePrefixedHash(id)` | `<RepoID>_<SHA>` node-id codec |
| `screenToWorld(x, y, t)` | Inverts the viewport transform for pointer events |
| `zoomAt(t, anchorX, anchorY, factor, min?, max?)` | Pointer-anchored multiplicative zoom with scale clamping |
