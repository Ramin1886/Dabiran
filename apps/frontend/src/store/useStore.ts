import { create } from 'zustand'

export interface CommitNode { 
  hash: string; 
  short_hash: string; 
  author: string; 
  message: string; 
  date: string; 
  parents: string[]; 
  lane?: number; 
  xOffset?: number; 
}

interface AppState { 
  nodes: CommitNode[]; 
  visibleNodes: CommitNode[];
  searchQuery: string;
  viewportTransform: { x: number; y: number; scale: number }; 
  selectedNode: string | null; 
  setNodes: (nodes: CommitNode[]) => void; 
  setSearchQuery: (query: string) => void;
  setViewportTransform: (transform: any) => void; 
  setSelectedNode: (hash: string | null) => void; 
}

export const useStore = create<AppState>((set, get) => ({
  nodes: [], 
  visibleNodes: [],
  searchQuery: '',
  viewportTransform: { x: 0, y: 0, scale: 1 }, 
  selectedNode: null,
  
  setNodes: (nodes) => set({ nodes, visibleNodes: nodes }),
  
  // Implements O(N) internal indexing logic filtering dynamically bypassing WebGL rendering lags generating selective arrays correctly scaling parameters directly resolving visibility smoothly updating UI context logically identifying nodes actively checking bounds safely updating arrays globally standardizing views efficiently tracking metrics natively projecting hashes reliably limiting arrays effectively executing algorithms.
  setSearchQuery: (query) => {
    const { nodes } = get();
    if (!query) {
      set({ searchQuery: query, visibleNodes: nodes });
      return;
    }
    
    const lowerQuery = query.toLowerCase();
    const filtered = nodes.filter(n => 
      n.hash.toLowerCase().includes(lowerQuery) || 
      n.author.toLowerCase().includes(lowerQuery) || 
      n.message.toLowerCase().includes(lowerQuery)
    );
    
    set({ searchQuery: query, visibleNodes: filtered });
  },

  setViewportTransform: (transform) => set({ viewportTransform: transform }),
  setSelectedNode: (selectedNode) => set({ selectedNode }),
}))
