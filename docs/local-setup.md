# Local Development Setup

This guide covers booting, compiling, and iterating on the monorepo locally.

## Prerequisites

| Tool | Version | Used for |
| :--- | :--- | :--- |
| Podman (or Docker) | any recent | PostgreSQL container, full-stack compose runs |
| Go | 1.22+ | `apps/backend` |
| Node.js | 20+ | `apps/frontend`, `packages/*` (npm workspaces) |

## 1. Database

The backend runs without PostgreSQL (it logs a warning and serves topology
from its local bare-repo cache), but persistence features need it.

```bash
cd infra/
podman-compose up -d postgres     # or: docker compose up -d postgres
```

Verification: PostgreSQL is reachable on `localhost:5432`
(`pg_isready -h localhost -U git_viz`).

## 2. Backend

```bash
cd apps/backend/
go mod download

# Optional overrides — see .env.example for the full list and defaults.
export DATABASE_URL="postgres://git_viz:secret_password@localhost:5432/git_interactive_history?sslmode=disable"

go run ./src/main.go
```

Verification: `curl http://localhost:8080/health` returns `OK`.

### Seeding a repository to visualize

The topology endpoint resolves each requested id first against the
`repositories` database table (clone-by-URL), then falls back to local bare
repositories in `apps/backend/repos/`. The quickest seed is a bare clone of
any repository:

```bash
git clone --bare <any-repo-url-or-path> apps/backend/repos/mock_1.git
```

The frontend loads `repo_ids=1` by default, which maps to `mock_1.git`.

### Running backend tests

```bash
cd apps/backend/
go vet ./...
go test ./...
```

## 3. Frontend

Dependencies are managed by npm workspaces from the **repository root** so
the shared `@git-viz/*` packages link correctly:

```bash
npm install            # repo root
npm run dev -w frontend
```

Verification: open `http://localhost:3000` — the HUD status line should
progress through *Authenticating… → Loading topology… → Loaded N commits*,
and the canvas should render the seeded repository.

| Frontend environment variable | Default | Purpose |
| :--- | :--- | :--- |
| `VITE_API_URL` | `http://localhost:8080` | Backend base URL for REST calls |

### Running frontend and package tests

```bash
npm test               # all workspaces (packages + frontend)
npm run build -w frontend
```

## 4. Full Stack via Compose

```bash
cd infra/
podman-compose up -d   # postgres + backend + frontend (frontend on :3000)
```

The frontend image is built from the repository root context because it needs
`packages/` and the workspace lockfile; the compose file already configures
this.

## Debugging Notes

> [!WARNING]
> When exercising the bare-git engine directly (outside compose), ensure the
> `apps/backend/repos/` directory exists and is writable — the engine creates
> and fetches bare clones there. The directory is git-ignored.

* The backend warns (instead of failing) when PostgreSQL is unreachable;
  watch its log for `WARNING: database unreachable`.
* The WebSocket relay is a peer relay: a single connected client receives no
  initial CRDT state — open two browser windows to observe synchronization.
