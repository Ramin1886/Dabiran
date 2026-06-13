import type { AuthResponse, CommitNode } from '@git-viz/shared-types';

/**
 * Base URL of the Go backend REST API. Override per environment via the
 * Vite env variable `VITE_API_URL`; defaults to the local dev backend
 * (docs/local-setup.md boots the Go daemon on port 8080).
 */
export const API_BASE: string =
  import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

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
