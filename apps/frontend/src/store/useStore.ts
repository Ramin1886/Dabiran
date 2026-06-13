import { create } from 'zustand';
import type { CommitNode } from '@git-viz/shared-types';
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
  setNodes: (nodes: CommitNode[]) => void;
  setSearchQuery: (query: string) => void;
  setViewportTransform: (transform: ViewportTransform) => void;
  setSelectedNode: (hash: string | null) => void;
  setDrawingState: (isActive: boolean) => void;
}

export const useStore = create<AppState>((set, get) => ({
  nodes: [],
  visibleNodes: [],
  searchQuery: '',
  viewportTransform: { x: 0, y: 0, scale: 1 },
  selectedNode: null,
  drawingState: false, // Flag activating map drawing pointers overriding defaults naturally.

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
    const childMap = new Map<string, number>();
    nodes.forEach((n) => {
      n.parents.forEach((p) => {
        childMap.set(p, (childMap.get(p) || 0) + 1);
      });
    });

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

  /** Persists the infinite-canvas pan/zoom transform applied by Canvas.tsx. */
  setViewportTransform: (transform) => set({ viewportTransform: transform }),

  /** Marks a commit hash as selected, opening the CommitPanel HUD window. */
  setSelectedNode: (selectedNode) => set({ selectedNode }),

  /** Toggles annotation drawing mode (disables panning while active). */
  setDrawingState: (drawingState) => set({ drawingState }),
}));
