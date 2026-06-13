import { describe, it, expect, beforeEach } from 'vitest';
import type { CommitNode } from '@git-viz/shared-types';
import { useStore } from './useStore';

/** Builds a wire-format CommitNode with sensible defaults for tests. */
function makeNode(overrides: Partial<CommitNode> = {}): CommitNode {
  return {
    hash: '1_aaaa',
    short_hash: 'aaaa',
    author: 'Alice',
    message: 'init',
    date: '2026-01-01T00:00:00Z',
    parents: [],
    lane: 0,
    x_offset: 0,
    repo_id: '1',
    tag: '',
    kind: 'commit',
    count: 1,
    ...overrides,
  };
}

// Topology: A is a split (two children B, C); D is a merge of B and C.
const nodeA = makeNode({ hash: '1_a', short_hash: 'a', message: 'root commit' });
const nodeB = makeNode({ hash: '1_b', short_hash: 'b', message: 'feature alpha work', parents: ['1_a'] });
const nodeC = makeNode({ hash: '1_c', short_hash: 'c', message: 'unrelated chore', parents: ['1_a'], lane: 1 });
const nodeD = makeNode({ hash: '1_d', short_hash: 'd', message: 'merge branches', parents: ['1_b', '1_c'] });

describe('useStore', () => {
  beforeEach(() => {
    useStore.setState({
      nodes: [],
      visibleNodes: [],
      searchQuery: '',
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
      token: null,
    });
  });

  it('setNodes populates both nodes and visibleNodes', () => {
    useStore.getState().setNodes([nodeA, nodeB]);
    expect(useStore.getState().nodes).toHaveLength(2);
    expect(useStore.getState().visibleNodes).toHaveLength(2);
  });

  describe('setSearchQuery (Selective Visibility filter)', () => {
    it('keeps textual matches AND retains split/merge skeleton nodes', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().setSearchQuery('alpha');

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).toContain('1_b'); // textual match
      expect(visible).toContain('1_a'); // split (two children) retained
      expect(visible).toContain('1_d'); // merge (two parents) retained
      expect(visible).not.toContain('1_c'); // plain non-match dropped
    });

    it('matches on hash and author as well as message', () => {
      useStore.getState().setNodes([nodeB, nodeC]);

      useStore.getState().setSearchQuery('1_c');
      expect(useStore.getState().visibleNodes.map((n) => n.hash)).toContain('1_c');

      useStore.getState().setSearchQuery('alice');
      expect(useStore.getState().visibleNodes).toHaveLength(2);
    });

    it('restores the full node list when the query is cleared', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().setSearchQuery('alpha');
      useStore.getState().setSearchQuery('');
      expect(useStore.getState().visibleNodes).toHaveLength(4);
    });

    it('re-applies the active filter after a topology refetch', () => {
      useStore.getState().setSearchQuery('alpha');
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).not.toContain('1_c');
      expect(visible).toContain('1_b');
    });
  });

  describe('setServerHits (deep-search result with retention)', () => {
    it('shows hit nodes plus retained split/merge skeleton nodes', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      // Server matched only nodeB; A (split) and D (merge) are retained.
      useStore.getState().setServerHits(['1_b']);

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).toContain('1_b'); // server hit
      expect(visible).toContain('1_a'); // split retained
      expect(visible).toContain('1_d'); // merge retained
      expect(visible).not.toContain('1_c'); // plain non-hit dropped
    });

    it('retains only the skeleton when there are no hits', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().setServerHits([]);

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).toEqual(expect.arrayContaining(['1_a', '1_d']));
      expect(visible).not.toContain('1_b');
      expect(visible).not.toContain('1_c');
    });

    it('does not mutate the searchQuery field', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().setSearchQuery('alpha');
      useStore.getState().setServerHits(['1_c']);
      expect(useStore.getState().searchQuery).toBe('alpha');
    });
  });

  it('setToken stores and clears the JWT access token', () => {
    expect(useStore.getState().token).toBeNull();
    useStore.getState().setToken('jwt-123');
    expect(useStore.getState().token).toBe('jwt-123');
    useStore.getState().setToken(null);
    expect(useStore.getState().token).toBeNull();
  });

  it('setViewportTransform persists the pan/zoom transform', () => {
    useStore.getState().setViewportTransform({ x: 12, y: -8, scale: 2.5 });
    expect(useStore.getState().viewportTransform).toEqual({ x: 12, y: -8, scale: 2.5 });
  });

  it('setSelectedNode and setDrawingState toggle interaction state', () => {
    useStore.getState().setSelectedNode('1_b');
    expect(useStore.getState().selectedNode).toBe('1_b');
    useStore.getState().setDrawingState(true);
    expect(useStore.getState().drawingState).toBe(true);
  });
});
