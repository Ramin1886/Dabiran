import { create } from 'zustand';
import type { CommitNode, DependencyLink } from '@git-viz/shared-types';
import type { ViewportTransform } from '@git-viz/utils';

/** Re-exported for convenience so canvas modules share one import site. */
export type { CommitNode };

interface AppState {
  nodes: CommitNode[];
  visibleNodes: CommitNode[];
  searchQuery: string;
  viewportTransform: ViewportTransform;
  selectedNode: string | null;
  drawingState: boolean;
  /**
   * JWT access token for authenticated API calls. Lifted out of App's boot
   * effect into the store so on-demand handlers (e.g. server-backed search)
   * can read it without prop-drilling. Null until login completes.
   */
  token: string | null;
  /**
   * Auto-generated cross-repository dependency links from the backend worker
   * (`GET /api/v1/dependency-links`). Rendered distinctly from user-drawn
   * annotation vectors; empty until the boot effect resolves them.
   */
  dependencyLinks: DependencyLink[];
  setNodes: (nodes: CommitNode[]) => void;
  setSearchQuery: (query: string) => void;
  setServerHits: (hashes: string[]) => void;
  setViewportTransform: (transform: ViewportTransform) => void;
  setSelectedNode: (hash: string | null) => void;
  setDrawingState: (isActive: boolean) => void;
  setToken: (token: string | null) => void;
  setDependencyLinks: (links: DependencyLink[]) => void;
}

/**
 * Builds the O(N) child-count map identifying split (branching) commits — a
 * hash maps to how many loaded nodes name it as a parent.
 *
 * @param nodes - the full loaded topology
 * @returns map of parent hash → number of children
 */
function buildChildMap(nodes: CommitNode[]): Map<string, number> {
  const childMap = new Map<string, number>();
  nodes.forEach((n) => {
    n.parents.forEach((p) => {
      childMap.set(p, (childMap.get(p) || 0) + 1);
    });
  });
  return childMap;
}

export const useStore = create<AppState>((set, get) => ({
  nodes: [],
  visibleNodes: [],
  searchQuery: '',
  viewportTransform: { x: 0, y: 0, scale: 1 },
  selectedNode: null,
  drawingState: false, // Flag activating map drawing pointers overriding defaults naturally.
  token: null,
  dependencyLinks: [],

  /**
   * Replaces the full topology dataset (wire-format CommitNode[] from
   * `GET /api/v1/topology`) and re-applies the active search filter so a
   * refetch never leaks stale visibility state.
   */
  setNodes: (nodes) => {
    set({ nodes, visibleNodes: nodes });
    const { searchQuery, setSearchQuery } = get();
    if (searchQuery) setSearchQuery(searchQuery);
  },

  /**
   * Selective Visibility filter: keeps nodes matching the query on
   * hash/author/message, and always retains structural split (multiple
   * children) and merge (multiple parents) commits so the filtered graph
   * preserves its topological skeleton (docs/features_doc.md §2,
   * "Contextual Branch Rule").
   */
  setSearchQuery: (query) => {
    const { nodes } = get();
    if (!query) {
      set({ searchQuery: query, visibleNodes: nodes });
      return;
    }

    // O(N) child-count map identifying split (branching) commits.
    const childMap = buildChildMap(nodes);

    const lowerQuery = query.toLowerCase();

    const filtered = nodes.filter((n) => {
      const isMatch =
        n.hash.toLowerCase().includes(lowerQuery) ||
        n.author.toLowerCase().includes(lowerQuery) ||
        n.message.toLowerCase().includes(lowerQuery);

      // Retain splits and merges so isolated branches keep their origin and
      // merge bounds visible regardless of the textual match.
      const isSplit = (childMap.get(n.hash) || 0) > 1;
      const isMerge = n.parents.length > 1;

      return isMatch || isSplit || isMerge;
    });

    set({ searchQuery: query, visibleNodes: filtered });
  },

  /**
   * Applies the result of a server-backed "deep" search across the full
   * index. The provided hashes are the commits the backend matched; we set
   * visibleNodes to the union of those hit nodes plus the structural
   * split/merge skeleton nodes, mirroring the retention rule used by the
   * client-side {@link setSearchQuery} filter so the filtered graph keeps its
   * topological skeleton.
   *
   * @param hashes - commit hashes returned by `GET /api/v1/search`
   */
  setServerHits: (hashes) => {
    const { nodes } = get();
    const hitSet = new Set(hashes);
    const childMap = buildChildMap(nodes);

    const filtered = nodes.filter((n) => {
      const isHit = hitSet.has(n.hash);
      const isSplit = (childMap.get(n.hash) || 0) > 1;
      const isMerge = n.parents.length > 1;
      return isHit || isSplit || isMerge;
    });

    set({ visibleNodes: filtered });
  },

  /** Persists the infinite-canvas pan/zoom transform applied by Canvas.tsx. */
  setViewportTransform: (transform) => set({ viewportTransform: transform }),

  /** Marks a commit hash as selected, opening the CommitPanel HUD window. */
  setSelectedNode: (selectedNode) => set({ selectedNode }),

  /** Toggles annotation drawing mode (disables panning while active). */
  setDrawingState: (drawingState) => set({ drawingState }),

  /** Stores the JWT access token after login for authenticated API calls. */
  setToken: (token) => set({ token }),

  /**
   * Stores the auto-generated cross-repository dependency links resolved from
   * the backend worker so the canvas can render them distinctly.
   */
  setDependencyLinks: (dependencyLinks) => set({ dependencyLinks }),
}));
