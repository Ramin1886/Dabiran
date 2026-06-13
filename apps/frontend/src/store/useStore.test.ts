import { describe, it, expect, beforeEach } from 'vitest';
import type { CommitNode, DependencyLink } from '@git-viz/shared-types';
import { useStore, applyFilters, laneList, authorList } from './useStore';

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
      serverHits: null,
      tagsOnly: false,
      hiddenLanes: [],
      hiddenAuthors: [],
      recompactLayout: false,
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
      token: null,
      dependencyLinks: [],
    });
  });

  it('toggleRecompact flips the flag without changing visibleNodes', () => {
    useStore.getState().setNodes([nodeA, nodeB]);
    const before = useStore.getState().visibleNodes;
    expect(useStore.getState().recompactLayout).toBe(false);
    useStore.getState().toggleRecompact();
    expect(useStore.getState().recompactLayout).toBe(true);
    // Recompaction only affects rendered coordinates, not the visible set.
    expect(useStore.getState().visibleNodes).toBe(before);
    useStore.getState().toggleRecompact();
    expect(useStore.getState().recompactLayout).toBe(false);
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

  it('setDependencyLinks stores the auto-generated cross-repo links', () => {
    expect(useStore.getState().dependencyLinks).toEqual([]);
    const links: DependencyLink[] = [
      { from_repo: '1', to_repo: '2', via: 'github.com/acme/shared', kind: 'go' },
    ];
    useStore.getState().setDependencyLinks(links);
    expect(useStore.getState().dependencyLinks).toEqual(links);
    useStore.getState().setDependencyLinks([]);
    expect(useStore.getState().dependencyLinks).toEqual([]);
  });

  describe('tagged-only visibility filter', () => {
    const tagged = makeNode({ hash: '1_t', short_hash: 't', message: 'release', tag: 'v1.0.0', lane: 1 });

    it('toggleTagsOnly keeps only tagged commits plus the split/merge skeleton', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD, tagged]);
      useStore.getState().toggleTagsOnly();

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(useStore.getState().tagsOnly).toBe(true);
      expect(visible).toContain('1_t'); // tagged
      expect(visible).toContain('1_a'); // split retained
      expect(visible).toContain('1_d'); // merge retained
      expect(visible).not.toContain('1_b'); // untagged plain commit dropped
      expect(visible).not.toContain('1_c');
    });

    it('toggling off restores the full topology', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD, tagged]);
      useStore.getState().toggleTagsOnly();
      useStore.getState().toggleTagsOnly();
      expect(useStore.getState().tagsOnly).toBe(false);
      expect(useStore.getState().visibleNodes).toHaveLength(5);
    });
  });

  describe('per-branch (lane) visibility', () => {
    it('toggleLane hides a lane but retains structural splits/merges', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      // Hide lane 1 (only nodeC, a plain commit).
      useStore.getState().toggleLane(1);
      expect(useStore.getState().hiddenLanes).toEqual([1]);

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).not.toContain('1_c'); // hidden lane, non-structural
      expect(visible).toContain('1_a'); // split retained
      expect(visible).toContain('1_d'); // merge retained
      expect(visible).toContain('1_b'); // lane 0 still visible
    });

    it('hiding a lane still retains that lane\'s split/merge bounds', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      // Hide lane 0 — A (split) and D (merge) live there but are structural.
      useStore.getState().toggleLane(0);
      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).toContain('1_a');
      expect(visible).toContain('1_d');
      expect(visible).not.toContain('1_b'); // plain lane-0 commit dropped
    });

    it('toggleLane is reversible and showAllLanes clears all hidden lanes', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().toggleLane(1);
      useStore.getState().toggleLane(1); // toggle back on
      expect(useStore.getState().hiddenLanes).toEqual([]);

      useStore.getState().toggleLane(0);
      useStore.getState().toggleLane(1);
      expect(useStore.getState().hiddenLanes).toEqual([0, 1]);
      useStore.getState().showAllLanes();
      expect(useStore.getState().hiddenLanes).toEqual([]);
      expect(useStore.getState().visibleNodes).toHaveLength(4);
    });
  });

  describe('per-author visibility', () => {
    // Bob authors a plain commit (lane 1); structural A/D remain Alice's.
    const bobCommit = makeNode({
      hash: '1_bob',
      short_hash: 'bob',
      author: 'Bob',
      message: 'bob plain work',
      parents: ['1_a'],
      lane: 1,
    });

    it('toggleAuthor hides that author\'s plain commits but retains structural splits/merges', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD, bobCommit]);
      useStore.getState().toggleAuthor('Bob');
      expect(useStore.getState().hiddenAuthors).toEqual(['Bob']);

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).not.toContain('1_bob'); // hidden author, non-structural
      expect(visible).toContain('1_a'); // split retained
      expect(visible).toContain('1_d'); // merge retained
      expect(visible).toContain('1_b'); // Alice's plain commit still visible
    });

    it('hiding an author still retains that author\'s structural bounds', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      // Hide Alice — every node is hers, but A (split) and D (merge) are structural.
      useStore.getState().toggleAuthor('Alice');
      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).toContain('1_a');
      expect(visible).toContain('1_d');
      expect(visible).not.toContain('1_b'); // plain Alice commit dropped
      expect(visible).not.toContain('1_c');
    });

    it('toggleAuthor is reversible and showAllAuthors clears all hidden authors', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD, bobCommit]);
      useStore.getState().toggleAuthor('Bob');
      useStore.getState().toggleAuthor('Bob'); // toggle back on
      expect(useStore.getState().hiddenAuthors).toEqual([]);

      useStore.getState().toggleAuthor('Bob');
      useStore.getState().toggleAuthor('Alice');
      expect(useStore.getState().hiddenAuthors).toEqual(['Alice', 'Bob']); // sorted
      useStore.getState().showAllAuthors();
      expect(useStore.getState().hiddenAuthors).toEqual([]);
      expect(useStore.getState().visibleNodes).toHaveLength(5);
    });
  });

  describe('applyFilters composition (pure)', () => {
    const tagged = makeNode({ hash: '1_t', short_hash: 't', message: 'alpha release', tag: 'v1.0.0', lane: 1 });

    it('returns all nodes when no filter is active', () => {
      const all = [nodeA, nodeB, nodeC, nodeD];
      expect(
        applyFilters(all, {
          searchQuery: '',
          serverHits: null,
          tagsOnly: false,
          hiddenLanes: [],
          hiddenAuthors: [],
        }),
      ).toBe(all);
    });

    it('composes tags-only AND search with structural retention', () => {
      const all = [nodeA, nodeB, nodeC, nodeD, tagged];
      const visible = applyFilters(all, {
        searchQuery: 'alpha',
        serverHits: null,
        tagsOnly: true,
        hiddenLanes: [],
        hiddenAuthors: [],
      }).map((n) => n.hash);
      // tagged AND matches "alpha"
      expect(visible).toContain('1_t');
      // nodeB matches "alpha"? message is "feature alpha work" -> yes, but it is
      // untagged so tags-only drops it.
      expect(visible).not.toContain('1_b');
      // structural retained regardless
      expect(visible).toContain('1_a');
      expect(visible).toContain('1_d');
    });

    it('composes a hidden author AND search with structural retention', () => {
      const bobCommit = makeNode({
        hash: '1_bob',
        short_hash: 'bob',
        author: 'Bob',
        message: 'alpha by bob',
        parents: ['1_a'],
        lane: 1,
      });
      const all = [nodeA, nodeB, nodeC, nodeD, bobCommit];
      const visible = applyFilters(all, {
        searchQuery: 'alpha',
        serverHits: null,
        tagsOnly: false,
        hiddenLanes: [],
        hiddenAuthors: ['Bob'],
      }).map((n) => n.hash);
      // nodeB matches "alpha" (message "feature alpha work") and is Alice's.
      expect(visible).toContain('1_b');
      // bobCommit matches "alpha" too, but Bob is hidden so it drops.
      expect(visible).not.toContain('1_bob');
      // structural retained regardless of the hidden author.
      expect(visible).toContain('1_a');
      expect(visible).toContain('1_d');
    });

    it('laneList returns sorted unique lanes', () => {
      expect(laneList([nodeA, nodeB, nodeC, nodeD])).toEqual([0, 1]);
      expect(laneList([])).toEqual([]);
    });

    it('authorList returns sorted unique authors', () => {
      const bob = makeNode({ hash: '1_bob', author: 'Bob' });
      const carol = makeNode({ hash: '1_car', author: 'Carol' });
      expect(authorList([carol, nodeA, bob, nodeB])).toEqual(['Alice', 'Bob', 'Carol']);
      expect(authorList([])).toEqual([]);
    });
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

  describe('applyView (restore a saved view)', () => {
    it('sets the viewport and every filter field atomically', () => {
      useStore.getState().setServerHits(['1_x']); // transient dimension to clear
      useStore.getState().applyView({
        viewport: { x: 10, y: 20, scale: 3 },
        searchQuery: 'alpha',
        tagsOnly: true,
        hiddenLanes: [1],
        hiddenAuthors: ['Bob'],
        recompactLayout: true,
      });

      const s = useStore.getState();
      expect(s.viewportTransform).toEqual({ x: 10, y: 20, scale: 3 });
      expect(s.searchQuery).toBe('alpha');
      expect(s.serverHits).toBeNull(); // transient result cleared
      expect(s.tagsOnly).toBe(true);
      expect(s.hiddenLanes).toEqual([1]);
      expect(s.hiddenAuthors).toEqual(['Bob']);
      expect(s.recompactLayout).toBe(true);
    });

    it('recomputes visibleNodes once so a restored searchQuery filters the set', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().applyView({
        viewport: { x: 0, y: 0, scale: 1 },
        searchQuery: 'alpha',
        tagsOnly: false,
        hiddenLanes: [],
        hiddenAuthors: [],
        recompactLayout: false,
      });

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).toContain('1_b'); // "feature alpha work" matches
      expect(visible).toContain('1_a'); // split retained
      expect(visible).toContain('1_d'); // merge retained
      expect(visible).not.toContain('1_c'); // plain non-match dropped
    });

    it('recomputes visibleNodes so restored hiddenLanes hide their plain nodes', () => {
      useStore.getState().setNodes([nodeA, nodeB, nodeC, nodeD]);
      useStore.getState().applyView({
        viewport: { x: 0, y: 0, scale: 1 },
        searchQuery: '',
        tagsOnly: false,
        hiddenLanes: [1],
        hiddenAuthors: [],
        recompactLayout: false,
      });

      const visible = useStore.getState().visibleNodes.map((n) => n.hash);
      expect(visible).not.toContain('1_c'); // lane 1, plain → hidden
      expect(visible).toContain('1_a'); // split retained
      expect(visible).toContain('1_d'); // merge retained
    });
  });
});
