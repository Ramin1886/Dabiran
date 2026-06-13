import { create } from 'zustand';
import type { CommitNode, DependencyLink } from '@git-viz/shared-types';
import type { ViewportTransform } from '@git-viz/utils';

/** Re-exported for convenience so canvas modules share one import site. */
export type { CommitNode };

interface AppState {
  nodes: CommitNode[];
  visibleNodes: CommitNode[];
  searchQuery: string;
  /**
   * Server-backed "deep" search result hashes, or null when no server search
   * is active. Takes precedence over the client-side {@link searchQuery} text
   * match while set; cleared the moment the user types again.
   */
  serverHits: string[] | null;
  /** When true, only tagged commits pass the filter (plus retained structure). */
  tagsOnly: boolean;
  /** Branch lanes the user has hidden via the visibility toggles. */
  hiddenLanes: number[];
  /** Authors the user has hidden via the per-author visibility toggles. */
  hiddenAuthors: string[];
  /**
   * When true, the canvas re-lays-out the visible subset client-side (via the
   * wasm math engine), recompacting branch lanes so filtering closes gaps,
   * instead of using the backend-assigned x_offset/lane fields.
   */
  recompactLayout: boolean;
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
  /** Toggles the "tagged commits only" visibility filter. */
  toggleTagsOnly: () => void;
  /** Hides the lane if visible, or reveals it if hidden. */
  toggleLane: (lane: number) => void;
  /** Clears all hidden lanes (reveals every branch). */
  showAllLanes: () => void;
  /** Hides the author if visible, or reveals it if hidden. */
  toggleAuthor: (author: string) => void;
  /** Clears all hidden authors (reveals every author). */
  showAllAuthors: () => void;
  /** Toggles client-side layout recompaction of the visible subset. */
  toggleRecompact: () => void;
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

/** The composable visibility filters applied to the topology. */
export interface FilterState {
  searchQuery: string;
  serverHits: string[] | null;
  tagsOnly: boolean;
  hiddenLanes: number[];
  hiddenAuthors: string[];
}

/**
 * Computes the visible node set from the full topology by composing every
 * active filter (search text or server hits, tagged-only, and per-branch lane
 * visibility) with the Contextual Branch Rule: structural split (multiple
 * children) and merge (multiple parents) commits are ALWAYS retained, so an
 * isolated or filtered branch keeps its origin and merge bounds visible and
 * its connections remain traceable (docs/features_doc.md §2).
 *
 * When no filter is active the full set is returned unchanged.
 *
 * @param nodes - the full loaded topology
 * @param f - the active filter state
 * @returns the nodes that should be rendered
 */
export function applyFilters(nodes: CommitNode[], f: FilterState): CommitNode[] {
  const anyActive =
    !!f.searchQuery ||
    f.serverHits !== null ||
    f.tagsOnly ||
    f.hiddenLanes.length > 0 ||
    f.hiddenAuthors.length > 0;
  if (!anyActive) return nodes;

  const childMap = buildChildMap(nodes);
  const hiddenLaneSet = new Set(f.hiddenLanes);
  const hiddenAuthorSet = new Set(f.hiddenAuthors);
  const hitSet = f.serverHits ? new Set(f.serverHits) : null;
  const lowerQuery = f.searchQuery.toLowerCase();

  return nodes.filter((n) => {
    // Contextual Branch Rule: always retain structural splits and merges.
    const isSplit = (childMap.get(n.hash) || 0) > 1;
    const isMerge = n.parents.length > 1;
    if (isSplit || isMerge) return true;

    // Branch (lane) visibility.
    if (hiddenLaneSet.has(n.lane)) return false;

    // Per-author visibility.
    if (hiddenAuthorSet.has(n.author)) return false;

    // Tagged-only filter.
    if (f.tagsOnly && !n.tag) return false;

    // Search dimension: a server-search result set takes precedence over the
    // instant client-side text match.
    if (hitSet) {
      if (!hitSet.has(n.hash)) return false;
    } else if (f.searchQuery) {
      const isMatch =
        n.hash.toLowerCase().includes(lowerQuery) ||
        n.author.toLowerCase().includes(lowerQuery) ||
        n.message.toLowerCase().includes(lowerQuery);
      if (!isMatch) return false;
    }

    return true;
  });
}

/**
 * Returns the sorted, de-duplicated list of branch lanes present in the
 * topology — the set of toggleable branches for the visibility HUD.
 *
 * @param nodes - the full loaded topology
 * @returns ascending unique lane indices
 */
export function laneList(nodes: CommitNode[]): number[] {
  return Array.from(new Set(nodes.map((n) => n.lane))).sort((a, b) => a - b);
}

/**
 * Returns the sorted, de-duplicated list of author names present in the
 * topology — the set of toggleable authors for the visibility HUD.
 *
 * @param nodes - the full loaded topology
 * @returns alphabetically sorted unique author names
 */
export function authorList(nodes: CommitNode[]): string[] {
  return Array.from(new Set(nodes.map((n) => n.author))).sort((a, b) => a.localeCompare(b));
}

export const useStore = create<AppState>((set, get) => {
  /** Recomputes visibleNodes from the current topology and filter state. */
  const recompute = () => {
    const { nodes, searchQuery, serverHits, tagsOnly, hiddenLanes, hiddenAuthors } = get();
    set({
      visibleNodes: applyFilters(nodes, {
        searchQuery,
        serverHits,
        tagsOnly,
        hiddenLanes,
        hiddenAuthors,
      }),
    });
  };

  return {
    nodes: [],
    visibleNodes: [],
    searchQuery: '',
    serverHits: null,
    tagsOnly: false,
    hiddenLanes: [],
    hiddenAuthors: [],
    recompactLayout: false,
    viewportTransform: { x: 0, y: 0, scale: 1 },
    selectedNode: null,
    drawingState: false, // Flag activating map drawing pointers overriding defaults naturally.
    token: null,
    dependencyLinks: [],

    /**
     * Replaces the full topology dataset (wire-format CommitNode[] from
     * `GET /api/v1/topology`) and re-applies the active filters so a refetch
     * never leaks stale visibility state.
     */
    setNodes: (nodes) => {
      set({ nodes });
      recompute();
    },

    /**
     * Instant client-side text filter on hash/author/message. Typing clears any
     * active server-search result so the live client filter takes over again.
     */
    setSearchQuery: (query) => {
      set({ searchQuery: query, serverHits: null });
      recompute();
    },

    /**
     * Applies the result of a server-backed "deep" search across the full
     * index. The provided hashes become the active search dimension (composed
     * with the tagged-only and branch filters and the structural retention
     * rule).
     *
     * @param hashes - commit hashes returned by `GET /api/v1/search`
     */
    setServerHits: (hashes) => {
      set({ serverHits: hashes });
      recompute();
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

    /** Toggles the "tagged commits only" filter and recomputes visibility. */
    toggleTagsOnly: () => {
      set({ tagsOnly: !get().tagsOnly });
      recompute();
    },

    /** Toggles a single branch lane's visibility and recomputes. */
    toggleLane: (lane) => {
      const hidden = new Set(get().hiddenLanes);
      if (hidden.has(lane)) {
        hidden.delete(lane);
      } else {
        hidden.add(lane);
      }
      set({ hiddenLanes: Array.from(hidden).sort((a, b) => a - b) });
      recompute();
    },

    /** Reveals every branch lane and recomputes. */
    showAllLanes: () => {
      set({ hiddenLanes: [] });
      recompute();
    },

    /** Toggles a single author's visibility and recomputes. */
    toggleAuthor: (author) => {
      const hidden = new Set(get().hiddenAuthors);
      if (hidden.has(author)) {
        hidden.delete(author);
      } else {
        hidden.add(author);
      }
      set({ hiddenAuthors: Array.from(hidden).sort((a, b) => a.localeCompare(b)) });
      recompute();
    },

    /** Reveals every author and recomputes. */
    showAllAuthors: () => {
      set({ hiddenAuthors: [] });
      recompute();
    },

    /**
     * Toggles client-side layout recompaction. This only affects rendered
     * coordinates (handled in NodeEngine), not which nodes are visible, so it
     * does not trigger a filter recompute.
     */
    toggleRecompact: () => set({ recompactLayout: !get().recompactLayout }),
  };
});
