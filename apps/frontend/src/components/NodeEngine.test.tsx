import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, act } from '@testing-library/react';
import type { CommitNode } from '@git-viz/shared-types';
import { NodeEngine, nodeLabel } from './NodeEngine';
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
});
