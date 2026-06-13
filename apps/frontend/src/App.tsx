import React, { useEffect, useRef, useState } from 'react';
import { repoMapRoom } from '@git-viz/shared-types';
import { InteractiveCanvas } from './components/Canvas';
import { CommitPanel } from './components/CommitPanel';
import { login, fetchTopology } from './api/client';
import { useStore } from './store/useStore';
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
 * search filter, drawing-mode toggle) above the WebGL canvas.
 */
export default function App() {
  const [status, setStatus] = useState<string>('Authenticating…');
  const searchQuery = useStore((state) => state.searchQuery);
  const setSearchQuery = useStore((state) => state.setSearchQuery);
  const drawingState = useStore((state) => state.drawingState);
  const setDrawingState = useStore((state) => state.setDrawingState);
  const didInit = useRef(false);

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

        setStatus('Loading topology…');
        const nodes = await fetchTopology(DEFAULT_REPO_IDS, auth.access_token);
        if (cancelled) return;

        useStore.getState().setNodes(nodes);
        setStatus(`Loaded ${nodes.length} commits`);
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
          <input
            type="search"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search commits…"
            aria-label="Search commits"
            style={{ ...hudControlStyle, width: '240px' }}
          />
          <button
            type="button"
            onClick={() => setDrawingState(!drawingState)}
            aria-pressed={drawingState}
            style={{
              ...hudControlStyle,
              cursor: 'pointer',
              fontWeight: 600,
              ...(drawingState
                ? {
                    border: '1px solid rgba(0, 229, 255, 0.6)',
                    color: '#00E5FF',
                    background: 'rgba(0, 229, 255, 0.12)',
                  }
                : {}),
            }}
          >
            {drawingState ? 'Drawing: ON' : 'Draw'}
          </button>
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
