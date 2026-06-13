import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { AuthResponse, CommitNode } from '@git-viz/shared-types';
import { login, fetchTopology, searchCommits, API_BASE } from './client';

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
});
