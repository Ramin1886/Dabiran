import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import type { CommitNode } from '@git-viz/shared-types';
import App from './App';
import { login, fetchTopology } from './api/client';
import { useStore } from './store/useStore';
import { useCRDT } from './store/useCRDT';

vi.mock('@pixi/react');
vi.mock('y-websocket');
vi.mock('./api/client', () => ({
  login: vi.fn(),
  fetchTopology: vi.fn(),
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
    ...overrides,
  };
}

describe('App', () => {
  beforeEach(() => {
    vi.mocked(login).mockResolvedValue({ access_token: 'tok', role: 'Team Member' });
    vi.mocked(fetchTopology).mockResolvedValue([makeNode()]);
    useStore.setState({
      nodes: [],
      visibleNodes: [],
      searchQuery: '',
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
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

  it('toggles drawing mode via the HUD button', async () => {
    render(<App />);
    await screen.findByText('Loaded 1 commits');

    const button = screen.getByRole('button', { name: 'Draw' });
    fireEvent.click(button);
    expect(useStore.getState().drawingState).toBe(true);
    expect(screen.getByRole('button', { name: 'Drawing: ON' })).toBeTruthy();
  });
});
