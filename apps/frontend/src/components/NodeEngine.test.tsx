import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import type { CommitNode } from '@git-viz/shared-types';
import { NodeEngine, nodeLabel, isAggregate, aggregateLabel } from './NodeEngine';
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
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
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
});
