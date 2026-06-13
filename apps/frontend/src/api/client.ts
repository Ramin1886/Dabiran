import type { AuthResponse, CanvasView, CommitNode, DependencyLink } from '@git-viz/shared-types';

/**
 * A single match from the server-backed deep search endpoint
 * (`GET /api/v1/search`). Mirrors the JSON shape in the shared contract;
 * declared locally because it is search-response-only and not part of the
 * canvas topology contract in @git-viz/shared-types.
 */
export interface SearchHit {
  hash: string;
  short_hash: string;
  author: string;
  message: string;
  repo_id: number;
  tag: string;
}

/**
 * Base URL of the Go backend REST API. Override per environment via the
 * Vite env variable `VITE_API_URL`; defaults to the local dev backend
 * (docs/local-setup.md boots the Go daemon on port 8080).
 */
export const API_BASE: string =
  import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

/**
 * A repository registered for the caller's team, as returned by
 * `GET /api/v1/repositories` (credentials are never included).
 */
export interface RepositorySummary {
  id: number;
  name: string;
  url: string;
}

/**
 * Lists the repositories registered for the caller's team.
 *
 * Calls `GET {API_BASE}/api/v1/repositories` with a bearer token and returns
 * the summaries (no credentials). Used on boot to discover which repositories
 * to load onto the canvas instead of relying on a hardcoded id.
 *
 * @param token - JWT access token obtained from {@link login}
 * @returns the team's repository summaries (possibly empty)
 * @throws Error when the response status is not OK
 */
export async function fetchRepositories(token: string): Promise<RepositorySummary[]> {
  const response = await fetch(`${API_BASE}/api/v1/repositories`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) {
    throw new Error(`Fetch repositories failed with status ${response.status}`);
  }
  const data = (await response.json()) as RepositorySummary[] | null;
  return data ?? [];
}

/**
 * Authenticates against the backend and returns the bearer token payload.
 *
 * Calls `GET {API_BASE}/api/v1/auth/login` (the backend's login endpoint,
 * see apps/backend/src/api/router.go) and parses the JSON body into the
 * shared {@link AuthResponse} contract.
 *
 * @returns the access token and RBAC role for the session
 * @throws Error when the response status is not OK (error message includes
 *   the HTTP status code so the HUD can surface it)
 */
export async function login(): Promise<AuthResponse> {
  const response = await fetch(`${API_BASE}/api/v1/auth/login`);
  if (!response.ok) {
    throw new Error(`Login failed with status ${response.status}`);
  }
  return (await response.json()) as AuthResponse;
}

/**
 * Fetches the unified commit topology for one or more repositories.
 *
 * Calls `GET {API_BASE}/api/v1/topology?repo_ids=<id,id,...>` with an
 * `Authorization: Bearer <token>` header and returns the wire-format
 * {@link CommitNode} array (snake_case fields, `<RepoID>_<SHA>` prefixed
 * hashes — see docs/apis_doc.md).
 *
 * @param repoIds - repository identifiers to merge onto the unified canvas
 * @param token - JWT access token obtained from {@link login}
 * @returns the chronologically sequenced commit nodes with layout fields
 * @throws Error when the response status is not OK
 */
export async function fetchTopology(
  repoIds: string[],
  token: string,
): Promise<CommitNode[]> {
  // Encode ids individually so the comma separators stay literal in the URL.
  const query = repoIds.map(encodeURIComponent).join(',');
  const response = await fetch(`${API_BASE}/api/v1/topology?repo_ids=${query}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) {
    throw new Error(`Topology fetch failed with status ${response.status}`);
  }
  return (await response.json()) as CommitNode[];
}

/**
 * Fetches the auto-generated cross-repository dependency links for a set of
 * repositories.
 *
 * Calls `GET {API_BASE}/api/v1/dependency-links?repo_ids=<id,id,...>` with an
 * `Authorization: Bearer <token>` header and returns the `links` array from the
 * `{ "links": [...] }` response envelope per the shared contract. These links
 * are produced by the backend worker (Go/npm dependency analysis) and are
 * rendered on the canvas distinctly from user-drawn annotation vectors.
 *
 * @param repoIds - repository identifiers to resolve dependency links for
 * @param token - JWT access token obtained from {@link login}
 * @returns the dependency links (empty array when the envelope omits `links`)
 * @throws Error when the response status is not OK (message includes the HTTP
 *   status so the HUD can surface it without blanking the canvas)
 */
export async function fetchDependencyLinks(
  repoIds: string[],
  token: string,
): Promise<DependencyLink[]> {
  // Encode ids individually so the comma separators stay literal in the URL.
  const query = repoIds.map(encodeURIComponent).join(',');
  const response = await fetch(
    `${API_BASE}/api/v1/dependency-links?repo_ids=${query}`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  if (!response.ok) {
    throw new Error(`Dependency links fetch failed with status ${response.status}`);
  }
  const body = (await response.json()) as { links: DependencyLink[] };
  return body.links ?? [];
}

/**
 * Runs a server-backed "deep" search across the full commit index.
 *
 * Calls `GET {API_BASE}/api/v1/search?q=<query>&repo_ids=<id,id,...>` with an
 * `Authorization: Bearer <token>` header and returns the `hits` array from the
 * `{ "hits": [...] }` response envelope per the shared search contract.
 *
 * @param query - free-text search string (matched server-side across the index)
 * @param repoIds - repository identifiers to scope the search to
 * @param token - JWT access token obtained from {@link login}
 * @returns the matching search hits (empty array when nothing matched)
 * @throws Error when the response status is not OK (message includes the
 *   HTTP status so callers can detect 503 and fall back to client filtering)
 */
export async function searchCommits(
  query: string,
  repoIds: string[],
  token: string,
): Promise<SearchHit[]> {
  // Encode ids individually so the comma separators stay literal in the URL.
  const repos = repoIds.map(encodeURIComponent).join(',');
  const response = await fetch(
    `${API_BASE}/api/v1/search?q=${encodeURIComponent(query)}&repo_ids=${repos}`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  if (!response.ok) {
    throw new Error(`Search failed with status ${response.status}`);
  }
  const body = (await response.json()) as { hits: SearchHit[] };
  return body.hits ?? [];
}

/**
 * Fetches the current user's saved canvas views.
 *
 * Calls `GET {API_BASE}/api/v1/views` with an `Authorization: Bearer <token>`
 * header and returns the `views` array from the `{ "views": [...] }` response
 * envelope per the shared contract. Each {@link CanvasView} carries an opaque,
 * frontend-owned JSON `state` string (serialized viewport + active filters).
 *
 * @param token - JWT access token obtained from {@link login}
 * @returns the saved canvas views (empty array when the envelope omits `views`)
 * @throws Error when the response status is not OK (message includes the HTTP
 *   status so the HUD can surface it without blanking the canvas)
 */
export async function fetchViews(token: string): Promise<CanvasView[]> {
  const response = await fetch(`${API_BASE}/api/v1/views`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) {
    throw new Error(`Views fetch failed with status ${response.status}`);
  }
  const body = (await response.json()) as { views: CanvasView[] };
  return body.views ?? [];
}

/**
 * Persists a new named canvas view (viewport + active filters snapshot).
 *
 * Calls `POST {API_BASE}/api/v1/views` with an `Authorization: Bearer <token>`
 * header and a JSON body `{ name, state }`, where `state` is the frontend-owned
 * serialized view-state string (see {@link CanvasView}). Returns the created
 * view (with its server-assigned id) from the `201` response.
 *
 * @param name - human-readable label the user gave the saved view
 * @param state - serialized view-state JSON string (opaque to the backend)
 * @param token - JWT access token obtained from {@link login}
 * @returns the created {@link CanvasView} including its server-assigned id
 * @throws Error when the response status is not OK
 */
export async function saveView(
  name: string,
  state: string,
  token: string,
): Promise<CanvasView> {
  const response = await fetch(`${API_BASE}/api/v1/views`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ name, state }),
  });
  if (!response.ok) {
    throw new Error(`View save failed with status ${response.status}`);
  }
  return (await response.json()) as CanvasView;
}

/**
 * Deletes a saved canvas view by its server-assigned id.
 *
 * Calls `DELETE {API_BASE}/api/v1/views/{id}` with an
 * `Authorization: Bearer <token>` header. The backend responds `204 No Content`
 * on success; this helper resolves with no value.
 *
 * @param id - server-assigned id of the view to delete
 * @param token - JWT access token obtained from {@link login}
 * @throws Error when the response status is not OK
 */
export async function deleteView(id: number, token: string): Promise<void> {
  const response = await fetch(`${API_BASE}/api/v1/views/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) {
    throw new Error(`View delete failed with status ${response.status}`);
  }
}
