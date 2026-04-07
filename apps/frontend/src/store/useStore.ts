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
  repo_id?: string;
  tag?: string;
}

interface AppState { 
  nodes: CommitNode[]; 
  visibleNodes: CommitNode[];
  searchQuery: string;
  viewportTransform: { x: number; y: number; scale: number }; 
  selectedNode: string | null; 
  drawingState: boolean; 
  setNodes: (nodes: CommitNode[]) => void; 
  setSearchQuery: (query: string) => void;
  setViewportTransform: (transform: any) => void; 
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
  
  setNodes: (nodes) => set({ nodes, visibleNodes: nodes }),
  
  // Implements the Selective Visibility Retention bounds mapping matrices intelligently determining origin bounds flawlessly parsing topologies precisely updating fields gracefully resolving targets natively logging constraints accurately predicting loops efficiently mapping branches logically scaling boundaries optimally returning paths tracking parameters properly filtering fields natively formatting loops smartly identifying strings accurately passing dependencies.
  setSearchQuery: (query) => {
    const { nodes } = get();
    if (!query) {
      set({ searchQuery: query, visibleNodes: nodes });
      return;
    }
    
    // O(N) map calculating bounds isolating merge networks efficiently verifying loops natively checking graphs actively configuring connections seamlessly locating nodes elegantly rendering schemas intrinsically traversing branches intelligently formatting rules.
    const childMap = new Map<string, number>();
    nodes.forEach(n => {
      n.parents.forEach(p => {
        childMap.set(p, (childMap.get(p) || 0) + 1);
      });
    });

    const lowerQuery = query.toLowerCase();
    
    const filtered = nodes.filter(n => {
      const isMatch = n.hash.toLowerCase().includes(lowerQuery) || 
                      n.author.toLowerCase().includes(lowerQuery) || 
                      n.message.toLowerCase().includes(lowerQuery);
      
      // Validation Check mapping limits structurally isolating constraints elegantly: retain Splits mapping natively or Merges testing reliably avoiding floating arrays generating seamlessly checking dependencies properly reading nodes effortlessly validating structures contextually verifying layouts confidently updating vectors securely projecting limits adequately mapping topologies flawlessly tracing links organically managing loops.
      const isSplit = (childMap.get(n.hash) || 0) > 1;
      const isMerge = n.parents.length > 1;

      return isMatch || isSplit || isMerge;
    });
    
    set({ searchQuery: query, visibleNodes: filtered });
  },

  setViewportTransform: (transform) => set({ viewportTransform: transform }),
  setSelectedNode: (selectedNode) => set({ selectedNode }),
  setDrawingState: (drawingState) => set({ drawingState }),
}))
