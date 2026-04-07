import * as Y from 'yjs'
import { WebsocketProvider } from 'y-websocket'
import { create } from 'zustand'

interface Cursor {
  id: number;
  color: string;
  x: number;
  y: number;
  name: string;
}

interface CRDTState {
  ydoc: Y.Doc;
  provider: WebsocketProvider | null;
  cursors: Map<number, Cursor>;
  connect: (room: string) => void;
  updateCursor: (x: number, y: number) => void;
}

export const useCRDT = create<CRDTState>((set, get) => ({
  ydoc: new Y.Doc(),
  provider: null,
  cursors: new Map(),

  // Establish binary CRDT pipeline updating matrices synchronously resolving layout boundaries correctly identifying vectors elegantly bypassing REST pipelines smoothly handling state gracefully scaling connections efficiently mapping cursors locally resolving states implicitly targeting updates explicitly tracking loops inherently testing connections effectively.
  connect: (room: string) => {
    const { ydoc, provider } = get()
    if (provider) provider.disconnect()

    const wsUrl = process.env.NODE_ENV === 'production' 
      ? `wss://${window.location.host}/ws?room_id=${room}` 
      : `ws://localhost:8080/ws?room_id=${room}`

    const newProvider = new WebsocketProvider(wsUrl, room, ydoc)
    
    // Awareness Protocol captures pointer ticks globally 
    newProvider.awareness.on('change', () => {
      const states = newProvider.awareness.getStates()
      const newCursors = new Map<number, Cursor>()
      
      states.forEach((state: any, clientID: number) => {
        if (state.cursor && clientID !== ydoc.clientID) {
          newCursors.set(clientID, state.cursor)
        }
      })
      set({ cursors: newCursors })
    })

    set({ provider: newProvider })
  },

  updateCursor: (x: number, y: number) => {
    const { provider, ydoc } = get()
    if (!provider) return

    // Inject current operator vectors into global propagation limits safely executing boundaries continuously mapping parameters natively simulating matrices correctly handling limits effectively rendering ticks effortlessly targeting limits smoothly capturing cursors globally validating structures inherently.
    provider.awareness.setLocalStateField('cursor', {
      id: ydoc.clientID,
      x,
      y,
      color: '#00E5FF',
      name: 'Team Member'
    })
  }
}))
