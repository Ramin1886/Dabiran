import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import type { CommitNode } from '@git-viz/shared-types';
import App from './App';
import { login, fetchTopology, fetchDependencyLinks, searchCommits } from './api/client';
import { useStore } from './store/useStore';
import { useCRDT } from './store/useCRDT';

vi.mock('@pixi/react');
vi.mock('y-websocket');
vi.mock('./api/client', () => ({
  login: vi.fn(),
  fetchTopology: vi.fn(),
  fetchDependencyLinks: vi.fn(),
  searchCommits: vi.fn(),
}));

/* eslint-disable @typescript-eslint/no-explicit-any */

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

describe('App', () => {
  beforeEach(() => {
    vi.mocked(login).mockReset().mockResolvedValue({ access_token: 'tok', role: 'Team Member' });
    vi.mocked(fetchTopology).mockReset().mockResolvedValue([makeNode()]);
    vi.mocked(fetchDependencyLinks).mockReset().mockResolvedValue([]);
    vi.mocked(searchCommits).mockReset().mockResolvedValue([]);
    useStore.setState({
      nodes: [],
      visibleNodes: [],
      searchQuery: '',
      serverHits: null,
      tagsOnly: false,
      hiddenLanes: [],
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
      token: null,
      dependencyLinks: [],
    });
  });

  it('boots login → topology fetch → store, and reports the load on the HUD status line', async () => {
    render(<App />);

    expect(await screen.findByText('Loaded 1 commits')).toBeTruthy();
    expect(login).toHaveBeenCalledTimes(1);
    expect(fetchTopology).toHaveBeenCalledWith(['1'], 'tok');
    expect(useStore.getState().nodes).toHaveLength(1);
  });

  it('connects the CRDT room repo_map_1 on mount', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const provider = useCRDT.getState().provider as any;
    expect(provider).not.toBeNull();
    expect(provider.url).toBe('ws://localhost:8080/ws');
    expect(provider.roomname).toBe('repo_map_1');
  });

  it('surfaces API failures on the HUD status line instead of crashing', async () => {
    vi.mocked(login).mockRejectedValueOnce(new Error('Login failed with status 503'));
    render(<App />);

    // findByText polls until the async rejection lands on the status line.
    const status = await screen.findByText('Error: Login failed with status 503');
    expect(status.getAttribute('data-testid')).toBe('hud-status');
  });

  it('binds the HUD search input to the store search filter', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const input = screen.getByLabelText('Search commits') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'alpha' } });

    expect(useStore.getState().searchQuery).toBe('alpha');
    expect(input.value).toBe('alpha');
  });

  it('fetches dependency links for the loaded repo ids and stores them after topology load', async () => {
    vi.mocked(fetchDependencyLinks).mockResolvedValueOnce([
      { from_repo: '1', to_repo: '2', via: 'github.com/acme/shared', kind: 'go' },
    ]);
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    // Resolved from the repo_id of the loaded topology nodes ('1').
    expect(fetchDependencyLinks).toHaveBeenCalledWith(['1'], 'tok');
    expect(useStore.getState().dependencyLinks).toHaveLength(1);
    expect(useStore.getState().dependencyLinks[0].via).toBe('github.com/acme/shared');
  });

  it('surfaces a dependency-link fetch failure on the HUD WITHOUT blanking the loaded graph', async () => {
    vi.mocked(fetchDependencyLinks).mockRejectedValueOnce(
      new Error('Dependency links fetch failed with status 502'),
    );
    render(<App />);

    // Status notes the failure but the loaded topology is preserved.
    expect(
      await screen.findByText(/dependency links unavailable/i),
    ).toBeTruthy();
    expect(useStore.getState().nodes).toHaveLength(1);
    expect(useStore.getState().dependencyLinks).toEqual([]);
  });

  it('lifts the auth token into the store after login', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');
    expect(useStore.getState().token).toBe('tok');
  });

  it('submitting the search box runs the server-backed deep search and drives the store filter', async () => {
    vi.mocked(searchCommits).mockResolvedValueOnce([
      { hash: '1_aaaa', short_hash: 'aaaa', author: 'Alice', message: 'init', repo_id: 1, tag: '' },
    ]);
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const input = screen.getByLabelText('Search commits') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'init' } });
    fireEvent.submit(input);

    // searchCommits called for the default repo ids using the lifted token.
    expect(searchCommits).toHaveBeenCalledWith('init', ['1'], 'tok');
    expect(await screen.findByText('Found 1 matches')).toBeTruthy();
    // The store filter reflects the hit (union with retained skeleton).
    expect(useStore.getState().visibleNodes.map((n) => n.hash)).toContain('1_aaaa');
  });

  it('falls back to client-side filtering and notes it when the search endpoint 503s', async () => {
    vi.mocked(searchCommits).mockRejectedValueOnce(new Error('Search failed with status 503'));
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const input = screen.getByLabelText('Search commits') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'init' } });
    fireEvent.submit(input);

    expect(
      await screen.findByText('Server search unavailable — showing local results'),
    ).toBeTruthy();
  });

  it('ignores empty server-search submissions', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const input = screen.getByLabelText('Search commits') as HTMLInputElement;
    fireEvent.submit(input);
    expect(searchCommits).not.toHaveBeenCalled();
  });

  it('toggles drawing mode via the HUD button', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const button = screen.getByRole('button', { name: 'Draw' });
    fireEvent.click(button);
    expect(useStore.getState().drawingState).toBe(true);
    expect(screen.getByRole('button', { name: 'Drawing: ON' })).toBeTruthy();
  });

  it('toggles the tagged-commits-only filter via the HUD button', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const button = screen.getByRole('button', { name: 'Tagged commits only' });
    expect(button.getAttribute('aria-pressed')).toBe('false');
    fireEvent.click(button);
    expect(useStore.getState().tagsOnly).toBe(true);
    expect(button.getAttribute('aria-pressed')).toBe('true');
  });

  it('opens the branches popover and hides a lane via its checkbox', async () => {
    // Two-lane topology so there are branches to toggle.
    vi.mocked(fetchTopology).mockReset().mockResolvedValue([
      makeNode({ hash: '1_a', lane: 0 }),
      makeNode({ hash: '1_b', lane: 1, parents: ['1_a'] }),
    ]);
    render(<App />);
    await screen.findByText('Loaded 2 commits');

    // Popover starts closed.
    expect(screen.queryByRole('menu', { name: 'Branch list' })).toBeNull();

    fireEvent.click(screen.getByRole('button', { name: 'Branch visibility' }));
    expect(screen.getByRole('menu', { name: 'Branch list' })).toBeTruthy();

    // Uncheck lane 1 to hide it.
    const lane1 = screen.getByLabelText('Branch lane 1') as HTMLInputElement;
    expect(lane1.checked).toBe(true);
    fireEvent.click(lane1);

    expect(useStore.getState().hiddenLanes).toEqual([1]);
    expect(useStore.getState().visibleNodes.map((n) => n.hash)).not.toContain('1_b');
  });
});
