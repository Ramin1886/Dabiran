/**
 * Shared data contracts between the Go backend and the TypeScript frontend.
 *
 * IMPORTANT: field names use snake_case because they mirror the JSON tags on
 * the Go structs in `apps/backend/src/gitengine/topology.go` and
 * `apps/backend/src/models/models.go`. Do not camelCase these — the wire
 * format is the contract (see docs/apis_doc.md "JSON Array Schema Example").
 */

/**
 * A single commit in the unified topology returned by `GET /api/v1/topology`.
 * `hash` and entries in `parents` use the `<RepoID>_<SHA>` prefix scheme so
 * nodes from different repositories never collide on the unified canvas.
 */
export interface CommitNode {
  hash: string;
  short_hash: string;
  author: string;
  message: string;
  /** RFC3339 timestamp string as serialized by Go's time.Time. */
  date: string;
  parents: string[];
  /** Horizontal branch lane assigned by the backend layout pass (Y axis). */
  lane: number;
  /** Chronological pixel offset assigned by the backend layout pass (X axis). */
  x_offset: number;
  repo_id: string;
  /** Tag name when the commit is tagged; empty string otherwise. Label priority: Tag > short_hash. */
  tag: string;
  /**
   * Node kind: `'commit'` for a real commit (the default), or `'aggregate'`
   * for a synthetic node collapsing a maximal run of linear commits when the
   * topology request supplies `max_nodes` and the extracted count exceeds it.
   */
  kind: 'commit' | 'aggregate';
  /** Underlying commit count: `1` for a real commit, the run length for an aggregate. */
  count: number;
}

/**
 * One full-text search hit returned by `GET /api/v1/search`. Field names are
 * snake_case to mirror the Go `SearchHit` JSON tags (apps/backend/src/search).
 */
export interface SearchHit {
  hash: string;
  short_hash: string;
  author: string;
  message: string;
  repo_id: string;
  /** Tag name when the indexed commit is tagged; empty string otherwise. */
  tag: string;
}

/**
 * An auto-generated cross-repository dependency link produced by the backend
 * worker (`GET /api/v1/dependency-links`). Field names are snake_case to
 * mirror the JSON tags on the Go/Rust worker struct — this is the wire format
 * contract shared with the backend agent.
 *
 * Unlike a user-drawn {@link AnnotationVector}, a dependency link references
 * repositories (not raw world coordinates): the frontend resolves
 * representative on-canvas nodes for `from_repo`/`to_repo` at render time and
 * stores the link as a synthetic `'dependency'` annotation on the canvas
 * (rendered distinctly — a dashed amber line — from collaborative vectors).
 */
export interface DependencyLink {
  /** repo_id (string) of the depending repository (source endpoint). */
  from_repo: string;
  /** repo_id (string) of the depended-upon repository (target endpoint). */
  to_repo: string;
  /** The module/package path establishing the dependency (label text). */
  via: string;
  /** Ecosystem that declared the dependency. */
  kind: 'go' | 'npm';
}

/**
 * A named snapshot of the canvas view state, persisted per user via the
 * `/api/v1/views` REST endpoints (apps/backend/src/api/views.go).
 *
 * `state` is an OPAQUE, frontend-owned JSON string — the serialized viewport
 * (pan/zoom) plus the active filters. The backend never parses it beyond
 * requiring it to be non-empty; the frontend serializes and deserializes its
 * own shape, so the contract here is just the persistence envelope.
 */
export interface CanvasView {
  /** Server-assigned numeric id (canvas_views.id). */
  id: number;
  /** Human-readable label the user gave the saved view. */
  name: string;
  /** Frontend-owned JSON string: serialized viewport + active filters. */
  state: string;
}

/** RBAC roles enforced by the backend JWT claims (see docs/features_doc.md §1). */
export type Role = 'Admin' | 'Team Owner' | 'Team Member';

/** Response payload of `/api/v1/auth/login` and the OAuth callback. */
export interface AuthResponse {
  access_token: string;
  role: Role;
}

/** Live cursor state broadcast through the Yjs awareness protocol. */
export interface CursorState {
  /** Yjs clientID of the cursor owner. */
  id: number;
  /** Hex CSS color used to render the remote cursor. */
  color: string;
  /** World-space X coordinate on the shared canvas. */
  x: number;
  /** World-space Y coordinate on the shared canvas. */
  y: number;
  /** Display name of the collaborator. */
  name: string;
}

/**
 * A manually drawn dependency line stored in the shared Yjs document
 * (`annotations` Y.Array). Coordinates are world-space canvas units so all
 * collaborators see the vector anchored identically regardless of viewport.
 */
export interface AnnotationVector {
  start_x: number;
  start_y: number;
  end_x: number;
  end_y: number;
  /** Hex CSS color of the line. */
  color: string;
  /** Yjs clientID of the author. */
  author_id: number;
}

/**
 * Builds the CRDT room identifier for a repository map, matching the
 * `?room_id=repo_map_{id}` connection parameter in docs/apis_doc.md.
 *
 * @param repoId - numeric or string repository identifier
 * @returns the canonical room id, e.g. `repo_map_1`
 */
export function repoMapRoom(repoId: number | string): string {
  return `repo_map_${repoId}`;
}
