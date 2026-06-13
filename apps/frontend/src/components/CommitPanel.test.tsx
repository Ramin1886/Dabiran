import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import type { CommitNode } from '@git-viz/shared-types';
import { CommitPanel } from './CommitPanel';
import { useStore } from '../store/useStore';

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

const node = makeNode({
  hash: '1_deadbeef',
  short_hash: 'deadbee',
  author: 'Alice',
  message: 'Ship the topology pipeline',
  parents: ['1_cafebabe', '1_baddcafe'],
  tag: 'v2.0.0',
});

describe('CommitPanel', () => {
  beforeEach(() => {
    useStore.setState({
      nodes: [node],
      visibleNodes: [node],
      searchQuery: '',
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
    });
  });

  it('renders nothing while no node is selected', () => {
    const { container } = render(<CommitPanel />);
    expect(container.firstChild).toBeNull();
  });

  it('shows the short hash, tag badge and parent lineage hashes of the selected commit', () => {
    useStore.setState({ selectedNode: '1_deadbeef' });
    render(<CommitPanel />);

    expect(screen.getByText('deadbee')).toBeTruthy();
    expect(screen.getByText('v2.0.0')).toBeTruthy();
    expect(screen.getByText('Ship the topology pipeline')).toBeTruthy();
    expect(screen.getByText('Alice')).toBeTruthy();
    expect(screen.getByText('1_cafebabe')).toBeTruthy();
    expect(screen.getByText('1_baddcafe')).toBeTruthy();
  });

  it('omits the tag badge and shows the origin placeholder for untagged root commits', () => {
    const root = makeNode({ hash: '1_root', short_hash: 'r00t' });
    useStore.setState({ nodes: [root], visibleNodes: [root], selectedNode: '1_root' });
    render(<CommitPanel />);

    expect(screen.getByText('r00t')).toBeTruthy();
    expect(screen.queryByText('v2.0.0')).toBeNull();
    expect(screen.getByText(/Origin Trajectory/)).toBeTruthy();
  });

  it('summarizes the N collapsed commits for a selected aggregate node', () => {
    const agg = makeNode({
      hash: '1_agg',
      short_hash: 'aggg',
      kind: 'aggregate',
      count: 9,
    });
    useStore.setState({ nodes: [agg], visibleNodes: [agg], selectedNode: '1_agg' });
    render(<CommitPanel />);

    // Aggregate variant of the panel, not the single-commit metadata view.
    expect(screen.getByTestId('aggregate-panel')).toBeTruthy();
    expect(screen.getByText('Collapsed Cluster')).toBeTruthy();
    expect(screen.getByText('9 commits')).toBeTruthy();
    expect(screen.getByText('AGGREGATE')).toBeTruthy();
    // The normal-commit header is absent for aggregates.
    expect(screen.queryByText('Node Target')).toBeNull();
  });
});
