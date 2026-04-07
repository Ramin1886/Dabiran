import { create } from 'zustand'

export interface CommitNode {
  hash: string
  short_hash: string
  author: string
  message: string
  date: string
  parents: string[]
  lane?: number // Graphical Y-axis position
  xOffset?: number // Graphical X-axis position
}

interface AppState {
  nodes: CommitNode[]
  viewportTransform: { x: number; y: number; scale: number }
  selectedNode: string | null
  
  setNodes: (nodes: CommitNode[]) => void
  setViewportTransform: (transform: { x: number; y: number; scale: number }) => void
  setSelectedNode: (hash: string | null) => void
}

export const useStore = create<AppState>((set) => ({
  nodes: [],
  viewportTransform: { x: 0, y: 0, scale: 1 },
  selectedNode: null,

  setNodes: (nodes) => set({ nodes }),
  setViewportTransform: (transform) => set({ viewportTransform: transform }),
  setSelectedNode: (selectedNode) => set({ selectedNode }),
}))
