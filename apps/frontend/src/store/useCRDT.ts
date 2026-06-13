import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';
import { create } from 'zustand';
import type { AnnotationVector, CursorState } from '@git-viz/shared-types';
import { useStore } from './useStore';
import { compactRoom } from '../api/client';

/** Name of the shared Y.Array holding persisted drawing vectors. */
const ANNOTATIONS_KEY = 'annotations';

/**
 * Resolves the y-websocket server base URL.
 *
 * Contract agreed with the backend: the provider is constructed as
 * `new WebsocketProvider(WS_BASE, room, ydoc)` where WS_BASE has NO query
 * string — y-websocket appends `/<room>` as a path segment and the backend
 * parses the room from the path. Dev talks to the local Go daemon; in
 * production the socket is derived from the serving host behind the HTTPS
 * proxy (wss://<host>/ws).
 *
 * @returns the websocket base URL, e.g. `ws://localhost:8080/ws`
 */
export function resolveWsBase(): string {
  return import.meta.env.PROD
    ? `wss://${window.location.host}/ws`
    : 'ws://localhost:8080/ws';
}

interface CRDTState {
  ydoc: Y.Doc;
  provider: WebsocketProvider | null;
  cursors: Map<number, CursorState>;
  annotations: AnnotationVector[];
  connect: (room: string) => void;
  updateCursor: (x: number, y: number) => void;
  addAnnotation: (vector: AnnotationVector) => void;
}

/**
 * Wires a Y.Doc's `annotations` array into the zustand state so React
 * re-renders whenever a local or remote drawing vector lands.
 *
 * @param ydoc - the shared document to observe
 * @param set - zustand setter
 * @returns the observed Y.Array for immediate reads
 */
function observeAnnotations(
  ydoc: Y.Doc,
  set: (partial: Partial<CRDTState>) => void,
): Y.Array<AnnotationVector> {
  const yAnnotations = ydoc.getArray<AnnotationVector>(ANNOTATIONS_KEY);
  yAnnotations.observe(() => set({ annotations: yAnnotations.toArray() }));
  return yAnnotations;
}

export const useCRDT = create<CRDTState>((set, get) => {
  const initialDoc = new Y.Doc();
  observeAnnotations(initialDoc, set);

  return {
    ydoc: initialDoc,
    provider: null,
    cursors: new Map(),
    annotations: [],

    /**
     * Connects to a CRDT room on a fresh Y.Doc, replacing any previous
     * provider/doc pair so state from a prior room never bleeds across.
     * Subscribes to awareness cursor broadcasts (world-space coordinates)
     * and to the shared `annotations` Y.Array.
     *
     * @param room - canonical room id, e.g. `repoMapRoom(1)` → `repo_map_1`
     */
    connect: (room: string) => {
      const { provider: previous } = get();
      if (previous) previous.destroy();

      const ydoc = new Y.Doc();
      const yAnnotations = observeAnnotations(ydoc, set);
      const newProvider = new WebsocketProvider(resolveWsBase(), room, ydoc);

      // Awareness Protocol: mirror every remote collaborator cursor tick.
      newProvider.awareness.on('change', () => {
        const states = newProvider.awareness.getStates();
        const newCursors = new Map<number, CursorState>();

        states.forEach((state: { cursor?: CursorState }, clientID: number) => {
          if (state.cursor && clientID !== ydoc.clientID) {
            newCursors.set(clientID, state.cursor);
          }
        });
        set({ cursors: newCursors });
      });

      // Compaction trigger: periodically compact the append-only log to PostgreSQL
      // once 50 mutations (local or remote) are made after initial synchronization.
      let isSynced = false;
      let updatesSinceCompacted = 0;

      newProvider.on('sync', (synced: boolean) => {
        isSynced = synced;
      });

      ydoc.on('update', () => {
        if (!isSynced) return;
        updatesSinceCompacted++;
        if (updatesSinceCompacted >= 50) {
          updatesSinceCompacted = 0;
          const token = useStore.getState().token;
          if (token) {
            const state = Y.encodeStateAsUpdate(ydoc);
            compactRoom(room, state, token).catch((err) => {
              console.error('Failed to compact room:', err);
            });
          }
        }
      });

      set({
        ydoc,
        provider: newProvider,
        cursors: new Map(),
        annotations: yAnnotations.toArray(),
      });
    },

    /**
     * Broadcasts the local pointer position through the awareness protocol.
     * Coordinates MUST already be world-space (converted via screenToWorld)
     * so collaborators see the cursor anchored to the graph, not the screen.
     *
     * @param x - world-space X coordinate
     * @param y - world-space Y coordinate
     */
    updateCursor: (x: number, y: number) => {
      const { provider, ydoc } = get();
      if (!provider) return;

      provider.awareness.setLocalStateField('cursor', {
        id: ydoc.clientID,
        x,
        y,
        color: '#00E5FF',
        name: 'Team Member',
      } satisfies CursorState);
    },

    /**
     * Persists a completed drawing vector into the shared `annotations`
     * Y.Array so it replicates to (and is rendered by) every collaborator.
     *
     * @param vector - world-space annotation line (see AnnotationVector)
     */
    addAnnotation: (vector: AnnotationVector) => {
      const { ydoc } = get();
      ydoc.getArray<AnnotationVector>(ANNOTATIONS_KEY).push([vector]);
    },
  };
});
