import { describe, expect, it } from 'vitest';
import { repoMapRoom, type AuthResponse, type CommitNode } from './index';

describe('repoMapRoom', () => {
  it('builds the canonical room id from a numeric repo id', () => {
    expect(repoMapRoom(1)).toBe('repo_map_1');
  });

  it('accepts string repo ids unchanged', () => {
    expect(repoMapRoom('42')).toBe('repo_map_42');
  });
});

describe('wire contract shapes', () => {
  it('CommitNode matches the backend JSON tags (snake_case)', () => {
    // Compile-time contract check: this literal must satisfy CommitNode
    // exactly as the Go backend serializes it (docs/apis_doc.md schema).
    const node: CommitNode = {
      hash: '1_a1b2c3d4e5f6g7h8',
      short_hash: 'a1b2c3d',
      author: 'Alice',
      message: 'Initial architectural commit',
      date: '2026-01-01T00:00:00Z',
      parents: ['1_previous_hash_string'],
      lane: 0,
      x_offset: 0,
      repo_id: '1',
      tag: 'v1.0.0',
    };
    expect(node.hash.startsWith(`${node.repo_id}_`)).toBe(true);
  });

  it('AuthResponse carries token and role', () => {
    const res: AuthResponse = { access_token: 'jwt', role: 'Team Owner' };
    expect(res.role).toBe('Team Owner');
  });
});
