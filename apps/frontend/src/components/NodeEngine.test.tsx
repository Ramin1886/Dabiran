import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import type { CommitNode, DependencyLink } from '@git-viz/shared-types';
import {
  NodeEngine,
  nodeLabel,
  isAggregate,
  aggregateLabel,
  representativeNode,
  truncateVia,
} from './NodeEngine';
import { useStore } from '../store/useStore';

vi.mock('@pixi/react');

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

const tagged = makeNode({ hash: '1_t', short_hash: 'tttt', tag: 'v1.0.0', x_offset: 120 });
const plain = makeNode({ hash: '1_p', short_hash: 'pppp', parents: ['1_t'], lane: 1, x_offset: 240, message: 'feature alpha' });

describe('NodeEngine', () => {
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
      dependencyLinks: [],
    });
  });

  it('nodeLabel prefers the tag text over the short hash', () => {
    expect(nodeLabel(tagged)).toBe('v1.0.0');
    expect(nodeLabel(plain)).toBe('pppp');
  });

  it('renders tag labels for tagged commits and short hashes otherwise', () => {
    act(() => useStore.getState().setNodes([tagged, plain]));
    render(<NodeEngine />);

    expect(screen.getByText('v1.0.0')).toBeTruthy();
    expect(screen.queryByText('tttt')).toBeNull();
    expect(screen.getByText('pppp')).toBeTruthy();
  });

  it('renders visibleNodes (the filtered subset), not the raw node list', () => {
    act(() => {
      useStore.getState().setNodes([tagged, plain]);
      useStore.getState().setSearchQuery('alpha');
    });
    render(<NodeEngine />);

    // 'alpha' matches only the plain node; the tagged one is neither a
    // split nor a merge, so it must not be rendered.
    expect(screen.getByText('pppp')).toBeTruthy();
    expect(screen.queryByText('v1.0.0')).toBeNull();
  });

  it('draws node circles plus connector lines between visible parent/child pairs', () => {
    act(() => useStore.getState().setNodes([tagged, plain]));
    render(<NodeEngine />);

    // 2 node circles + 1 connector (plain → tagged parent).
    expect(screen.getAllByTestId('pixi-graphics')).toHaveLength(3);
  });

  describe('viewport culling', () => {
    // jsdom defaults the window to 1024x768; the visible world rect at the
    // identity transform spans roughly x:[-80,1104], y:[-80,848] (1-row pad).
    const onScreen = makeNode({ hash: '1_on', short_hash: 'onon', x_offset: 100, lane: 0 });
    const offScreen = makeNode({
      hash: '1_off',
      short_hash: 'offf',
      x_offset: 50_000, // way beyond the right edge of the visible rect
      lane: 0,
    });

    it('renders on-screen nodes but culls off-screen ones', () => {
      act(() => useStore.getState().setNodes([onScreen, offScreen]));
      render(<NodeEngine />);

      expect(screen.getByText('onon')).toBeTruthy();
      expect(screen.queryByText('offf')).toBeNull();
    });

    it('still renders everything for a small fully-visible graph', () => {
      act(() => useStore.getState().setNodes([tagged, plain]));
      render(<NodeEngine />);

      // Same 3 graphics as the un-culled baseline — culling is a no-op here.
      expect(screen.getAllByTestId('pixi-graphics')).toHaveLength(3);
    });

    it('culls a connector when neither endpoint nor its span touch the rect', () => {
      // Both nodes off-screen, far apart but both to the right — segment bbox
      // never overlaps the visible rect, so the connector is culled too.
      const farChild = makeNode({
        hash: '1_fc',
        short_hash: 'farc',
        x_offset: 60_000,
        lane: 0,
        parents: ['1_off'],
      });
      act(() => useStore.getState().setNodes([offScreen, farChild]));
      render(<NodeEngine />);

      // Nothing rendered: both nodes and the connector are culled.
      expect(screen.queryAllByTestId('pixi-graphics')).toHaveLength(0);
    });
  });

  describe('semantic-zoom aggregate rendering', () => {
    const agg = makeNode({
      hash: '1_agg',
      short_hash: 'aggg',
      x_offset: 200,
      lane: 0,
      kind: 'aggregate',
      count: 7,
    });

    it('isAggregate/aggregateLabel reflect the contract fields', () => {
      expect(isAggregate(agg)).toBe(true);
      expect(isAggregate(plain)).toBe(false);
      expect(aggregateLabel(agg)).toBe('+7');
    });

    it('renders the aggregate cluster with a "+N" count label', () => {
      act(() => useStore.getState().setNodes([agg]));
      render(<NodeEngine />);

      expect(screen.getByText('+7')).toBeTruthy();
      // It is drawn as a glyph (one graphics) — its hash short label is not
      // shown; the count label replaces it.
      expect(screen.queryByText('aggg')).toBeNull();
    });

    it('still draws connectors to/from an aggregate node', () => {
      const child = makeNode({
        hash: '1_child',
        short_hash: 'chld',
        x_offset: 320,
        lane: 1,
        parents: ['1_agg'],
      });
      act(() => useStore.getState().setNodes([agg, child]));
      render(<NodeEngine />);

      // 2 node glyphs + 1 connector (child → aggregate parent).
      expect(screen.getAllByTestId('pixi-graphics')).toHaveLength(3);
    });
  });

  describe('cross-repo dependency links', () => {
    const repo1 = makeNode({ hash: '1_a', short_hash: 'r1aa', repo_id: '1', x_offset: 100, lane: 0 });
    const repo1Newer = makeNode({ hash: '1_b', short_hash: 'r1bb', repo_id: '1', x_offset: 300, lane: 0, parents: ['1_a'] });
    const repo2 = makeNode({ hash: '2_a', short_hash: 'r2aa', repo_id: '2', x_offset: 200, lane: 1 });

    const link: DependencyLink = {
      from_repo: '1',
      to_repo: '2',
      via: 'github.com/acme/shared',
      kind: 'go',
    };

    it('representativeNode picks the latest (max x_offset) node of a repo, null when absent', () => {
      expect(representativeNode([repo1, repo1Newer, repo2], '1')).toBe(repo1Newer);
      expect(representativeNode([repo1, repo2], '2')).toBe(repo2);
      expect(representativeNode([repo1, repo2], '9')).toBeNull();
    });

    it('truncateVia shortens long module paths with an ellipsis', () => {
      expect(truncateVia('@acme/x')).toBe('@acme/x');
      const long = 'github.com/acme/really-long-module-path/internal';
      expect(truncateVia(long).length).toBeLessThanOrEqual(24);
      expect(truncateVia(long).endsWith('…')).toBe(true);
    });

    it('renders a dashed line + via label when both repos have visible nodes', () => {
      act(() => {
        useStore.getState().setNodes([repo1, repo1Newer, repo2]);
        useStore.getState().setDependencyLinks([link]);
      });
      render(<NodeEngine />);

      // The via label is emitted as Pixi Text.
      expect(screen.getByText('github.com/acme/shared')).toBeTruthy();
      // A dependency-link Graphics is added on top of the 3 node/connector
      // graphics (3 nodes + 2 commit connectors among repo nodes ... here:
      // 3 node glyphs + 1 connector (1_b→1_a) + 1 dependency line = 5).
      expect(screen.getAllByTestId('pixi-graphics')).toHaveLength(5);
    });

    it('skips a dependency link when the to_repo has no visible node', () => {
      act(() => {
        useStore.getState().setNodes([repo1, repo1Newer]); // no repo 2 node
        useStore.getState().setDependencyLinks([link]);
      });
      render(<NodeEngine />);

      // No via label, and no extra dependency Graphics: just 2 node glyphs +
      // 1 connector (1_b→1_a).
      expect(screen.queryByText('github.com/acme/shared')).toBeNull();
      expect(screen.getAllByTestId('pixi-graphics')).toHaveLength(3);
    });

    it('skips a dependency link when the from_repo has no visible node', () => {
      act(() => {
        useStore.getState().setNodes([repo2]); // no repo 1 node
        useStore.getState().setDependencyLinks([link]);
      });
      render(<NodeEngine />);

      expect(screen.queryByText('github.com/acme/shared')).toBeNull();
      // Only the single repo2 node glyph.
      expect(screen.getAllByTestId('pixi-graphics')).toHaveLength(1);
    });
  });

  describe('recompact layout (wasm engine)', () => {
    // A node placed far off-screen by its backend x_offset. Its child shares
    // the same date, so client recompaction maps both back to the origin.
    const root = makeNode({ hash: '1_a', short_hash: 'rroot', x_offset: 0, lane: 0 });
    const far = makeNode({
      hash: '1_far',
      short_hash: 'rfar',
      x_offset: 50_000, // off-screen under the backend coordinates
      lane: 0,
      parents: ['1_a'],
    });

    it('uses backend coordinates when recompact is off (far node culled)', () => {
      act(() => useStore.getState().setNodes([root, far]));
      render(<NodeEngine />);
      expect(screen.queryByText('rfar')).toBeNull();
    });

    it('recomputes positions client-side when recompact is on (far node returns)', () => {
      act(() => {
        useStore.getState().setNodes([root, far]);
        useStore.setState({ recompactLayout: true });
      });
      render(<NodeEngine />);
      // Recompaction lays both nodes near the origin (shared date), so the
      // formerly off-screen node is now rendered.
      expect(screen.getByText('rfar')).toBeTruthy();
    });
  });
});
