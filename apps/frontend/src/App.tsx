import React, { useEffect, useMemo, useRef, useState } from 'react';
import { repoMapRoom } from '@git-viz/shared-types';
import type { CanvasView } from '@git-viz/shared-types';
import { InteractiveCanvas } from './components/Canvas';
import { CommitPanel } from './components/CommitPanel';
import {
  login,
  fetchTopology,
  fetchDependencyLinks,
  fetchRepositories,
  searchCommits,
  fetchViews,
  saveView,
  deleteView,
} from './api/client';
import { useStore, laneList, authorList } from './store/useStore';
import {
  captureViewState,
  serializeViewState,
  parseViewState,
} from './views/viewState';
import { useCRDT } from './store/useCRDT';

/** Repository ids loaded onto the unified canvas on boot. */
const DEFAULT_REPO_IDS = ['1'];

/** Shared glassmorphism styling for the HUD controls. */
const hudControlStyle: React.CSSProperties = {
  background: 'rgba(15, 23, 42, 0.6)',
  backdropFilter: 'blur(12px)',
  WebkitBackdropFilter: 'blur(12px)',
  border: '1px solid rgba(255, 255, 255, 0.08)',
  borderRadius: '10px',
  color: '#e2e8f0',
  fontFamily: '"Inter", sans-serif',
  fontSize: '0.85rem',
  padding: '8px 14px',
  outline: 'none',
};

/**
 * Main App: boots the data pipeline (login → topology fetch → store), joins
 * the CRDT collaboration room, and renders the HUD (title, status line,
 * search filter, drawing-mode toggle, tagged-only filter, and per-branch
 * visibility toggles) above the WebGL canvas.
 */
/** Glassmorphism style for an active (toggled-on) HUD control. */
const hudActiveStyle: React.CSSProperties = {
  border: '1px solid rgba(0, 229, 255, 0.6)',
  color: '#00E5FF',
  background: 'rgba(0, 229, 255, 0.12)',
};

export default function App() {
  const [status, setStatus] = useState<string>('Authenticating…');
  const [branchesOpen, setBranchesOpen] = useState(false);
  const [authorsOpen, setAuthorsOpen] = useState(false);
  const [viewsOpen, setViewsOpen] = useState(false);
  const [views, setViews] = useState<CanvasView[]>([]);
  const [viewName, setViewName] = useState('');
  const searchQuery = useStore((state) => state.searchQuery);
  const setSearchQuery = useStore((state) => state.setSearchQuery);
  const setServerHits = useStore((state) => state.setServerHits);
  const drawingState = useStore((state) => state.drawingState);
  const setDrawingState = useStore((state) => state.setDrawingState);
  const nodes = useStore((state) => state.nodes);
  const tagsOnly = useStore((state) => state.tagsOnly);
  const toggleTagsOnly = useStore((state) => state.toggleTagsOnly);
  const hiddenLanes = useStore((state) => state.hiddenLanes);
  const toggleLane = useStore((state) => state.toggleLane);
  const showAllLanes = useStore((state) => state.showAllLanes);
  const hiddenAuthors = useStore((state) => state.hiddenAuthors);
  const toggleAuthor = useStore((state) => state.toggleAuthor);
  const showAllAuthors = useStore((state) => state.showAllAuthors);
  const recompactLayout = useStore((state) => state.recompactLayout);
  const toggleRecompact = useStore((state) => state.toggleRecompact);
  const didInit = useRef(false);
  /**
   * Repository ids currently loaded onto the canvas — discovered from the
   * backend on boot (all of the team's registered repos), falling back to
   * DEFAULT_REPO_IDS. Held in a ref so on-demand handlers (server search) use
   * the live set without re-rendering.
   */
  const activeRepoIds = useRef<string[]>(DEFAULT_REPO_IDS);

  /** Sorted unique branch lanes present in the loaded topology. */
  const lanes = useMemo(() => laneList(nodes), [nodes]);

  /** Sorted unique author names present in the loaded topology. */
  const authors = useMemo(() => authorList(nodes), [nodes]);

  /**
   * On-submit "deep" server search across the full index. Calls
   * {@link searchCommits} for the loaded repo ids using the token lifted into
   * the store, then drives the store filter with the returned hit hashes
   * (union of hits + retained split/merge skeleton). Surfaces progress and
   * errors on the HUD status line; if the endpoint 503s, falls back to the
   * client-side filter (already applied on keystroke) and notes it.
   *
   * @param query - the submitted search text (empty submits are ignored)
   */
  const handleServerSearch = async (query: string): Promise<void> => {
    if (!query.trim()) return;
    const token = useStore.getState().token;
    if (!token) {
      setStatus('Search unavailable: not authenticated');
      return;
    }

    setStatus('Searching index…');
    try {
      const hits = await searchCommits(query, activeRepoIds.current, token);
      setServerHits(hits.map((h) => h.hash));
      setStatus(`Found ${hits.length} matches`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (message.includes('503')) {
        // Server unavailable — the client-side filter is already in effect.
        setStatus('Server search unavailable — showing local results');
      } else {
        setStatus(`Error: ${message}`);
      }
    }
  };

  /**
   * Best-effort refresh of the saved-views list from the backend. A fetch
   * failure is surfaced on the HUD status line and must NOT blank the graph or
   * throw into render, so callers can fire-and-forget it.
   *
   * @param token - JWT access token to authenticate the request
   */
  const refreshViews = async (token: string): Promise<void> => {
    try {
      setViews(await fetchViews(token));
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setStatus(`Saved views unavailable: ${message}`);
    }
  };

  /**
   * Captures and serializes the current viewport + filters and persists them as
   * a named view, then refreshes the list and clears the name input. Empty
   * names are ignored; errors surface on the HUD status line (never thrown).
   */
  const handleSaveView = async (): Promise<void> => {
    const name = viewName.trim();
    if (!name) return;
    const token = useStore.getState().token;
    if (!token) {
      setStatus('Save unavailable: not authenticated');
      return;
    }
    try {
      const state = serializeViewState(captureViewState());
      await saveView(name, state, token);
      setViewName('');
      await refreshViews(token);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setStatus(`Error: ${message}`);
    }
  };

  /**
   * Loads a saved view: safely parses its stored state and, when valid, applies
   * it to the store (viewport + filters) in one atomic update. A malformed
   * stored state is ignored with a HUD note rather than crashing the canvas.
   *
   * @param view - the saved view to restore
   */
  const handleLoadView = (view: CanvasView): void => {
    const parsed = parseViewState(view.state);
    if (!parsed) {
      setStatus(`Could not load view "${view.name}": corrupt state`);
      return;
    }
    useStore.getState().applyView(parsed);
  };

  /**
   * Deletes a saved view by id, then refreshes the list. Errors surface on the
   * HUD status line (never thrown into render).
   *
   * @param view - the saved view to delete
   */
  const handleDeleteView = async (view: CanvasView): Promise<void> => {
    const token = useStore.getState().token;
    if (!token) {
      setStatus('Delete unavailable: not authenticated');
      return;
    }
    try {
      await deleteView(view.id, token);
      await refreshViews(token);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      setStatus(`Error: ${message}`);
    }
  };

  /**
   * Boot effect: authenticate, fetch the commit topology for the default
   * repositories into the store, and connect the Yjs room. Errors are
   * surfaced on the HUD status line instead of crashing the canvas.
   */
  useEffect(() => {
    if (didInit.current) return;
    didInit.current = true;

    let cancelled = false;

    (async () => {
      try {
        setStatus('Authenticating…');
        const auth = await login();
        if (cancelled) return;

        // Lift the token into the store so on-demand handlers (server search)
        // can read it without prop-drilling through the boot effect.
        useStore.getState().setToken(auth.access_token);

        // Best-effort load of the saved-views list. A failure here only notes
        // the HUD status line (handled in refreshViews) and never blanks the
        // graph or blocks the topology load below.
        void refreshViews(auth.access_token);

        // Discover the team's registered repositories and load all of them,
        // so the canvas shows whatever is registered rather than a fixed id.
        // Falls back to DEFAULT_REPO_IDS when none are registered or the
        // lookup fails.
        try {
          const repos = await fetchRepositories(auth.access_token);
          if (cancelled) return;
          if (repos.length) activeRepoIds.current = repos.map((r) => String(r.id));
        } catch {
          // Keep the default repo ids; topology load below still proceeds.
        }

        setStatus('Loading topology…');
        const nodes = await fetchTopology(activeRepoIds.current, auth.access_token);
        if (cancelled) return;

        useStore.getState().setNodes(nodes);
        setStatus(`Loaded ${nodes.length} commits`);

        // Auto-generated cross-repo dependency links (backend worker). Resolve
        // the repo ids actually present in the loaded topology so links match
        // visible nodes; fall back to the requested ids when none are present.
        const loadedRepoIds = Array.from(new Set(nodes.map((n) => n.repo_id)));
        const linkRepoIds = loadedRepoIds.length ? loadedRepoIds : activeRepoIds.current;
        try {
          const links = await fetchDependencyLinks(linkRepoIds, auth.access_token);
          if (cancelled) return;
          useStore.getState().setDependencyLinks(links);
        } catch (linkErr) {
          // A dependency-link fetch failure must NOT blank the graph: keep the
          // loaded topology and only annotate the HUD status line.
          if (cancelled) return;
          const message =
            linkErr instanceof Error ? linkErr.message : String(linkErr);
          setStatus(`Loaded ${nodes.length} commits (dependency links unavailable: ${message})`);
        }
      } catch (err) {
        if (!cancelled) {
          setStatus(`Error: ${err instanceof Error ? err.message : String(err)}`);
        }
      }
    })();

    // Join the collaborative CRDT room for the default repository map.
    useCRDT.getState().connect(repoMapRoom(1));

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh', backgroundColor: '#0f172a', overflow: 'hidden' }}>

      {/* HUD Layer — title, status, search and drawing controls. */}
      <div style={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        padding: '20px 40px',
        background: 'linear-gradient(to bottom, rgba(15, 23, 42, 0.9) 0%, rgba(15, 23, 42, 0) 100%)',
        zIndex: 10,
        pointerEvents: 'none',
        display: 'flex',
        alignItems: 'center',
        gap: '20px',
      }}>
        <h1 style={{
          margin: 0,
          fontSize: '1.5rem',
          color: '#e2e8f0',
          letterSpacing: '-0.02em',
          fontFamily: '"Inter", sans-serif',
          fontWeight: 800,
        }}>
          Git VR <span style={{ color: '#00E5FF' }}>Interactive</span>
        </h1>

        {/* Interactive HUD controls re-enable pointer events explicitly. */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', pointerEvents: 'auto' }}>
          {/* Instant client-side filter on keystroke; on submit (Enter) runs
              the server-backed deep search across the full index. */}
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void handleServerSearch(searchQuery);
            }}
          >
            <input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search commits…"
              aria-label="Search commits"
              style={{ ...hudControlStyle, width: '240px' }}
            />
          </form>
          <button
            type="button"
            onClick={() => setDrawingState(!drawingState)}
            aria-pressed={drawingState}
            style={{
              ...hudControlStyle,
              cursor: 'pointer',
              fontWeight: 600,
              ...(drawingState ? hudActiveStyle : {}),
            }}
          >
            {drawingState ? 'Drawing: ON' : 'Draw'}
          </button>

          {/* Tagged-commits-only visibility filter. */}
          <button
            type="button"
            onClick={() => toggleTagsOnly()}
            aria-pressed={tagsOnly}
            aria-label="Tagged commits only"
            style={{
              ...hudControlStyle,
              cursor: 'pointer',
              fontWeight: 600,
              ...(tagsOnly ? hudActiveStyle : {}),
            }}
          >
            Tags only
          </button>

          {/* Client-side layout recompaction (wasm engine): re-lays-out the
              visible subset so filtering closes branch-lane gaps. */}
          <button
            type="button"
            onClick={() => toggleRecompact()}
            aria-pressed={recompactLayout}
            aria-label="Recompact layout"
            style={{
              ...hudControlStyle,
              cursor: 'pointer',
              fontWeight: 600,
              ...(recompactLayout ? hudActiveStyle : {}),
            }}
          >
            Recompact
          </button>

          {/* Per-branch (lane) visibility toggles. Splits and merges stay
              visible regardless, so isolated branches keep their bounds. */}
          <div style={{ position: 'relative' }}>
            <button
              type="button"
              onClick={() => setBranchesOpen((open) => !open)}
              aria-expanded={branchesOpen}
              aria-label="Branch visibility"
              style={{
                ...hudControlStyle,
                cursor: 'pointer',
                fontWeight: 600,
                ...(hiddenLanes.length > 0 ? hudActiveStyle : {}),
              }}
            >
              Branches{hiddenLanes.length > 0 ? ` (${hiddenLanes.length} hidden)` : ''} ▾
            </button>

            {branchesOpen && (
              <div
                role="menu"
                aria-label="Branch list"
                style={{
                  position: 'absolute',
                  top: 'calc(100% + 8px)',
                  left: 0,
                  minWidth: '180px',
                  maxHeight: '320px',
                  overflowY: 'auto',
                  background: 'rgba(15, 23, 42, 0.92)',
                  backdropFilter: 'blur(16px)',
                  WebkitBackdropFilter: 'blur(16px)',
                  border: '1px solid rgba(255, 255, 255, 0.1)',
                  borderRadius: '12px',
                  padding: '10px',
                  boxShadow: '0 20px 40px -12px rgba(0,0,0,0.6)',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    marginBottom: '6px',
                    paddingBottom: '6px',
                    borderBottom: '1px solid rgba(255,255,255,0.08)',
                  }}
                >
                  <span style={{ fontSize: '0.75rem', color: '#94a3b8', fontWeight: 600 }}>
                    {lanes.length} branches
                  </span>
                  <button
                    type="button"
                    onClick={() => showAllLanes()}
                    style={{
                      ...hudControlStyle,
                      padding: '2px 8px',
                      fontSize: '0.7rem',
                      cursor: 'pointer',
                    }}
                  >
                    Show all
                  </button>
                </div>
                {lanes.map((lane) => {
                  const hidden = hiddenLanes.includes(lane);
                  return (
                    <label
                      key={lane}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        padding: '4px 2px',
                        fontSize: '0.85rem',
                        color: hidden ? '#64748b' : '#e2e8f0',
                        cursor: 'pointer',
                      }}
                    >
                      <input
                        type="checkbox"
                        checked={!hidden}
                        onChange={() => toggleLane(lane)}
                        aria-label={`Branch lane ${lane}`}
                      />
                      Branch {lane}
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          {/* Per-author visibility toggles. Splits and merges stay visible
              regardless, so isolated branches keep their bounds. */}
          <div style={{ position: 'relative' }}>
            <button
              type="button"
              onClick={() => setAuthorsOpen((open) => !open)}
              aria-expanded={authorsOpen}
              aria-label="Author visibility"
              style={{
                ...hudControlStyle,
                cursor: 'pointer',
                fontWeight: 600,
                ...(hiddenAuthors.length > 0 ? hudActiveStyle : {}),
              }}
            >
              Authors{hiddenAuthors.length > 0 ? ` (${hiddenAuthors.length} hidden)` : ''} ▾
            </button>

            {authorsOpen && (
              <div
                role="menu"
                aria-label="Author list"
                style={{
                  position: 'absolute',
                  top: 'calc(100% + 8px)',
                  left: 0,
                  minWidth: '180px',
                  maxHeight: '320px',
                  overflowY: 'auto',
                  background: 'rgba(15, 23, 42, 0.92)',
                  backdropFilter: 'blur(16px)',
                  WebkitBackdropFilter: 'blur(16px)',
                  border: '1px solid rgba(255, 255, 255, 0.1)',
                  borderRadius: '12px',
                  padding: '10px',
                  boxShadow: '0 20px 40px -12px rgba(0,0,0,0.6)',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    marginBottom: '6px',
                    paddingBottom: '6px',
                    borderBottom: '1px solid rgba(255,255,255,0.08)',
                  }}
                >
                  <span style={{ fontSize: '0.75rem', color: '#94a3b8', fontWeight: 600 }}>
                    {authors.length} authors
                  </span>
                  <button
                    type="button"
                    onClick={() => showAllAuthors()}
                    style={{
                      ...hudControlStyle,
                      padding: '2px 8px',
                      fontSize: '0.7rem',
                      cursor: 'pointer',
                    }}
                  >
                    Show all
                  </button>
                </div>
                {authors.map((name) => {
                  const hidden = hiddenAuthors.includes(name);
                  return (
                    <label
                      key={name}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        padding: '4px 2px',
                        fontSize: '0.85rem',
                        color: hidden ? '#64748b' : '#e2e8f0',
                        cursor: 'pointer',
                      }}
                    >
                      <input
                        type="checkbox"
                        checked={!hidden}
                        onChange={() => toggleAuthor(name)}
                        aria-label={`Author ${name}`}
                      />
                      {name}
                    </label>
                  );
                })}
              </div>
            )}
          </div>

          {/* Saved canvas views: save/load/delete named snapshots of the
              current viewport + active filters. */}
          <div style={{ position: 'relative' }}>
            <button
              type="button"
              onClick={() => setViewsOpen((open) => !open)}
              aria-expanded={viewsOpen}
              aria-label="Saved views"
              style={{
                ...hudControlStyle,
                cursor: 'pointer',
                fontWeight: 600,
              }}
            >
              Views{views.length > 0 ? ` (${views.length})` : ''} ▾
            </button>

            {viewsOpen && (
              <div
                role="menu"
                aria-label="Saved views"
                style={{
                  position: 'absolute',
                  top: 'calc(100% + 8px)',
                  left: 0,
                  minWidth: '220px',
                  maxHeight: '320px',
                  overflowY: 'auto',
                  background: 'rgba(15, 23, 42, 0.92)',
                  backdropFilter: 'blur(16px)',
                  WebkitBackdropFilter: 'blur(16px)',
                  border: '1px solid rgba(255, 255, 255, 0.1)',
                  borderRadius: '12px',
                  padding: '10px',
                  boxShadow: '0 20px 40px -12px rgba(0,0,0,0.6)',
                }}
              >
                {/* Save row: name the current view and persist it. */}
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    void handleSaveView();
                  }}
                  style={{
                    display: 'flex',
                    gap: '8px',
                    marginBottom: '8px',
                    paddingBottom: '8px',
                    borderBottom: '1px solid rgba(255,255,255,0.08)',
                  }}
                >
                  <input
                    type="text"
                    value={viewName}
                    onChange={(e) => setViewName(e.target.value)}
                    placeholder="View name…"
                    aria-label="View name"
                    style={{ ...hudControlStyle, flex: 1, minWidth: 0, padding: '4px 8px' }}
                  />
                  <button
                    type="submit"
                    style={{
                      ...hudControlStyle,
                      padding: '4px 10px',
                      fontSize: '0.8rem',
                      fontWeight: 600,
                      cursor: 'pointer',
                    }}
                  >
                    Save
                  </button>
                </form>

                {views.length === 0 ? (
                  <span style={{ fontSize: '0.75rem', color: '#94a3b8' }}>
                    No saved views yet
                  </span>
                ) : (
                  views.map((view) => (
                    <div
                      key={view.id}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        padding: '4px 2px',
                      }}
                    >
                      <button
                        type="button"
                        onClick={() => handleLoadView(view)}
                        aria-label={`Load view ${view.name}`}
                        style={{
                          ...hudControlStyle,
                          flex: 1,
                          minWidth: 0,
                          textAlign: 'left',
                          padding: '4px 8px',
                          fontSize: '0.85rem',
                          cursor: 'pointer',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {view.name}
                      </button>
                      <button
                        type="button"
                        onClick={() => void handleDeleteView(view)}
                        aria-label={`Delete view ${view.name}`}
                        style={{
                          ...hudControlStyle,
                          padding: '4px 8px',
                          fontSize: '0.8rem',
                          cursor: 'pointer',
                        }}
                      >
                        ✕
                      </button>
                    </div>
                  ))
                )}
              </div>
            )}
          </div>
        </div>

        {/* Load/error status line. */}
        <span
          data-testid="hud-status"
          style={{
            marginLeft: 'auto',
            fontFamily: '"Inter", sans-serif',
            fontSize: '0.8rem',
            color: status.startsWith('Error') ? '#f87171' : '#94a3b8',
            fontWeight: 500,
          }}
        >
          {status}
        </span>
      </div>

      {/* WebGL Canvas Pipeline */}
      <InteractiveCanvas />

      {/* Contextual Interaction Floating Windows */}
      <CommitPanel />

    </div>
  );
}
