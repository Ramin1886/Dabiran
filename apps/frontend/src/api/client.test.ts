import { describe, it, expect, vi, beforeEach } from 'vitest';
import type {
  AuthResponse,
  CanvasView,
  CommitNode,
  DependencyLink,
} from '@git-viz/shared-types';
import {
  login,
  fetchTopology,
  fetchDependencyLinks,
  searchCommits,
  fetchViews,
  saveView,
  deleteView,
  API_BASE,
} from './client';

const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

/** Builds a minimal OK fetch Response stub resolving to the given JSON. */
function okResponse(body: unknown) {
  return { ok: true, status: 200, json: async () => body };
}

describe('api client', () => {
  beforeEach(() => {
    fetchMock.mockReset();
  });

  it('defaults API_BASE to the local dev backend', () => {
    expect(API_BASE).toBe('http://localhost:8080');
  });

  describe('login', () => {
    it('GETs /api/v1/auth/login and returns the AuthResponse payload', async () => {
      const payload: AuthResponse = { access_token: 'jwt-token', role: 'Team Owner' };
      fetchMock.mockResolvedValueOnce(okResponse(payload));

      const result = await login();

      expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/auth/login');
      expect(result).toEqual(payload);
    });

    it('propagates non-OK statuses as errors', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 401, json: async () => ({}) });
      await expect(login()).rejects.toThrow('Login failed with status 401');
    });
  });

  describe('fetchTopology', () => {
    it('GETs /api/v1/topology with comma-separated repo_ids and a Bearer header', async () => {
      const nodes: CommitNode[] = [];
      fetchMock.mockResolvedValueOnce(okResponse(nodes));

      const result = await fetchTopology(['1', '2'], 'my-token');

      expect(fetchMock).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/topology?repo_ids=1,2',
        { headers: { Authorization: 'Bearer my-token' } },
      );
      expect(result).toEqual(nodes);
    });

    it('propagates non-OK statuses as errors', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 403, json: async () => ({}) });
      await expect(fetchTopology(['1'], 'bad')).rejects.toThrow(
        'Topology fetch failed with status 403',
      );
    });

    it('propagates network failures', async () => {
      fetchMock.mockRejectedValueOnce(new TypeError('network down'));
      await expect(fetchTopology(['1'], 't')).rejects.toThrow('network down');
    });
  });

  describe('fetchDependencyLinks', () => {
    it('GETs /api/v1/dependency-links with comma-separated repo_ids and a Bearer header, returning links', async () => {
      const links: DependencyLink[] = [
        { from_repo: '1', to_repo: '2', via: 'github.com/acme/shared', kind: 'go' },
      ];
      fetchMock.mockResolvedValueOnce(okResponse({ links }));

      const result = await fetchDependencyLinks(['1', '2'], 'my-token');

      expect(fetchMock).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/dependency-links?repo_ids=1,2',
        { headers: { Authorization: 'Bearer my-token' } },
      );
      expect(result).toEqual(links);
    });

    it('returns an empty array when the response omits links', async () => {
      fetchMock.mockResolvedValueOnce(okResponse({}));
      const result = await fetchDependencyLinks(['1'], 't');
      expect(result).toEqual([]);
    });

    it('propagates non-OK statuses as errors', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 502, json: async () => ({}) });
      await expect(fetchDependencyLinks(['1'], 't')).rejects.toThrow(
        'Dependency links fetch failed with status 502',
      );
    });

    it('propagates network failures', async () => {
      fetchMock.mockRejectedValueOnce(new TypeError('offline'));
      await expect(fetchDependencyLinks(['1'], 't')).rejects.toThrow('offline');
    });
  });

  describe('searchCommits', () => {
    it('GETs /api/v1/search with encoded query + repo_ids and a Bearer header, returning hits', async () => {
      const hits = [
        { hash: '1_abc', short_hash: 'abc', author: 'Alice', message: 'fix bug', repo_id: 1, tag: '' },
      ];
      fetchMock.mockResolvedValueOnce(okResponse({ hits }));

      const result = await searchCommits('fix bug', ['1', '2'], 'my-token');

      expect(fetchMock).toHaveBeenCalledWith(
        'http://localhost:8080/api/v1/search?q=fix%20bug&repo_ids=1,2',
        { headers: { Authorization: 'Bearer my-token' } },
      );
      expect(result).toEqual(hits);
    });

    it('returns an empty array when the response omits hits', async () => {
      fetchMock.mockResolvedValueOnce(okResponse({}));
      const result = await searchCommits('q', ['1'], 't');
      expect(result).toEqual([]);
    });

    it('propagates non-OK statuses as errors (incl. 503 for fallback detection)', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 503, json: async () => ({}) });
      await expect(searchCommits('q', ['1'], 't')).rejects.toThrow(
        'Search failed with status 503',
      );
    });

    it('propagates network failures', async () => {
      fetchMock.mockRejectedValueOnce(new TypeError('offline'));
      await expect(searchCommits('q', ['1'], 't')).rejects.toThrow('offline');
    });
  });

  describe('fetchViews', () => {
    it('GETs /api/v1/views with a Bearer header and returns the views envelope', async () => {
      const views: CanvasView[] = [{ id: 1, name: 'Overview', state: '{}' }];
      fetchMock.mockResolvedValueOnce(okResponse({ views }));

      const result = await fetchViews('my-token');

      expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/views', {
        headers: { Authorization: 'Bearer my-token' },
      });
      expect(result).toEqual(views);
    });

    it('returns an empty array when the response omits views', async () => {
      fetchMock.mockResolvedValueOnce(okResponse({}));
      const result = await fetchViews('t');
      expect(result).toEqual([]);
    });

    it('propagates non-OK statuses as errors', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 500, json: async () => ({}) });
      await expect(fetchViews('t')).rejects.toThrow('Views fetch failed with status 500');
    });
  });

  describe('saveView', () => {
    it('POSTs /api/v1/views with a name+state body and a Bearer header, returning the view', async () => {
      const created: CanvasView = { id: 7, name: 'Release', state: '{"a":1}' };
      fetchMock.mockResolvedValueOnce({ ok: true, status: 201, json: async () => created });

      const result = await saveView('Release', '{"a":1}', 'my-token');

      expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/views', {
        method: 'POST',
        headers: {
          Authorization: 'Bearer my-token',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: 'Release', state: '{"a":1}' }),
      });
      expect(result).toEqual(created);
    });

    it('propagates non-OK statuses as errors', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 422, json: async () => ({}) });
      await expect(saveView('n', 's', 't')).rejects.toThrow('View save failed with status 422');
    });
  });

  describe('deleteView', () => {
    it('DELETEs /api/v1/views/{id} with a Bearer header', async () => {
      fetchMock.mockResolvedValueOnce({ ok: true, status: 204, json: async () => ({}) });

      await deleteView(42, 'my-token');

      expect(fetchMock).toHaveBeenCalledWith('http://localhost:8080/api/v1/views/42', {
        method: 'DELETE',
        headers: { Authorization: 'Bearer my-token' },
      });
    });

    it('propagates non-OK statuses as errors', async () => {
      fetchMock.mockResolvedValueOnce({ ok: false, status: 404, json: async () => ({}) });
      await expect(deleteView(1, 't')).rejects.toThrow('View delete failed with status 404');
    });
  });
});
