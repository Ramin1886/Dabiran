import { useEffect } from 'react';
import * as Y from 'yjs';
import { WebsocketProvider } from 'y-websocket';
import { useStore } from './useStore';

export const ydoc = new Y.Doc();
// In-browser CRDT memory array holding custom diagram notes and explicit cross-links
export const annotationsArray = ydoc.getArray('annotations');

export const useCRDT = (roomName: string) => {
  const setNodes = useStore(state => state.setNodes); // Could apply manual links here dynamically
  
  useEffect(() => {
    // Standard connection to Go WebSocket Relay serving as the Yjs Broadcast Backend
    const provider = new WebsocketProvider('ws://localhost:8080/ws', roomName, ydoc);

    // Sync Presence (Real-time Cursors)
    const awareness = provider.awareness;
    
    // Assign a random color payload for the active user cursor
    awareness.setLocalStateField('user', {
      name: 'User_' + Math.floor(Math.random() * 100),
      color: '#' + Math.floor(Math.random()*16777215).toString(16),
      clientX: 0,
      clientY: 0
    });

    const handlePointerMove = (e: PointerEvent) => {
      awareness.setLocalStateField('user', {
         ...awareness.getLocalState()?.user,
         clientX: e.clientX,
         clientY: e.clientY
      });
    };

    window.addEventListener('pointermove', handlePointerMove);

    return () => {
      window.removeEventListener('pointermove', handlePointerMove);
      provider.destroy();
    };
  }, [roomName]);
};
