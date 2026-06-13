import { useStore } from '../store/useStore';

/**
 * The frontend-owned, serializable snapshot persisted as a {@link CanvasView}'s
 * opaque `state` string. Captures the infinite-canvas viewport (pan/zoom) plus
 * every active visibility filter so loading a saved view restores both where
 * the user was looking and what they were looking at.
 *
 * The server-backed deep-search result set (`serverHits`) is intentionally
 * NOT persisted: it is a transient, query-derived dimension, and applying a
 * view clears it so the restored text query drives the live client filter.
 */
export interface CanvasViewState {
  /** Persisted infinite-canvas pan/zoom transform. */
  viewport: { x: number; y: number; scale: number };
  /** Instant client-side text filter (hash/author/message). */
  searchQuery: string;
  /** When true, only tagged commits pass the filter. */
  tagsOnly: boolean;
  /** Branch lanes hidden via the visibility toggles. */
  hiddenLanes: number[];
  /** Authors hidden via the per-author visibility toggles. */
  hiddenAuthors: string[];
  /** When true, the canvas recompacts the visible subset client-side. */
  recompactLayout: boolean;
}

/**
 * Captures the current viewport + active filter state from the global store
 * into a serializable {@link CanvasViewState}. Reads a one-shot snapshot via
 * `useStore.getState()` so it can be called from event handlers outside React.
 *
 * Arrays are copied so the captured snapshot never aliases live store state.
 *
 * @returns the current view-state snapshot
 */
export function captureViewState(): CanvasViewState {
  const s = useStore.getState();
  return {
    viewport: {
      x: s.viewportTransform.x,
      y: s.viewportTransform.y,
      scale: s.viewportTransform.scale,
    },
    searchQuery: s.searchQuery,
    tagsOnly: s.tagsOnly,
    hiddenLanes: [...s.hiddenLanes],
    hiddenAuthors: [...s.hiddenAuthors],
    recompactLayout: s.recompactLayout,
  };
}

/**
 * Serializes a {@link CanvasViewState} into the JSON string persisted as a
 * {@link CanvasView}'s opaque `state` field.
 *
 * @param s - the view-state snapshot to serialize
 * @returns the JSON string for the persistence envelope
 */
export function serializeViewState(s: CanvasViewState): string {
  return JSON.stringify(s);
}

/** Type guard for a finite number (rejects NaN/Infinity and non-numbers). */
function isFiniteNumber(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v);
}

/**
 * Safely parses a persisted `state` JSON string back into a
 * {@link CanvasViewState}. Validates every field's shape (viewport numbers,
 * booleans, arrays) and coerces the arrays element-wise, dropping any
 * mistyped entries. Returns `null` — never throws — on malformed JSON or any
 * missing/mistyped top-level field, so a corrupt stored view can never crash
 * the render path.
 *
 * @param json - the opaque `state` string from a {@link CanvasView}
 * @returns the parsed view-state, or `null` when invalid
 */
export function parseViewState(json: string): CanvasViewState | null {
  let raw: unknown;
  try {
    raw = JSON.parse(json);
  } catch {
    return null;
  }

  if (typeof raw !== 'object' || raw === null) return null;
  const o = raw as Record<string, unknown>;

  const vp = o.viewport;
  if (typeof vp !== 'object' || vp === null) return null;
  const v = vp as Record<string, unknown>;
  if (!isFiniteNumber(v.x) || !isFiniteNumber(v.y) || !isFiniteNumber(v.scale)) {
    return null;
  }

  if (typeof o.searchQuery !== 'string') return null;
  if (typeof o.tagsOnly !== 'boolean') return null;
  if (typeof o.recompactLayout !== 'boolean') return null;
  if (!Array.isArray(o.hiddenLanes)) return null;
  if (!Array.isArray(o.hiddenAuthors)) return null;

  // Coerce arrays element-wise, dropping any mistyped entries.
  const hiddenLanes = o.hiddenLanes.filter(isFiniteNumber);
  const hiddenAuthors = o.hiddenAuthors.filter(
    (a): a is string => typeof a === 'string',
  );

  return {
    viewport: { x: v.x, y: v.y, scale: v.scale },
    searchQuery: o.searchQuery,
    tagsOnly: o.tagsOnly,
    hiddenLanes,
    hiddenAuthors,
    recompactLayout: o.recompactLayout,
  };
}
