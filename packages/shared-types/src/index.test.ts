import { describe, expect, it } from 'vitest';
import {
  repoMapRoom,
  type AuthResponse,
  type CommitNode,
  type DependencyLink,
  type SearchHit,
} from './index';

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
      kind: 'commit',
      count: 1,
    };
    expect(node.hash.startsWith(`${node.repo_id}_`)).toBe(true);
  });

  it('CommitNode supports aggregate nodes (max_nodes collapse)', () => {
    const agg: CommitNode = {
      hash: 'agg_1_oldsha_newsha',
      short_hash: '+5',
      author: 'Alice',
      message: '5 commits collapsed',
      date: '2026-01-05T00:00:00Z',
      parents: ['1_external_parent'],
      lane: 0,
      x_offset: 25,
      repo_id: '1',
      tag: '',
      kind: 'aggregate',
      count: 5,
    };
    expect(agg.kind).toBe('aggregate');
    expect(agg.count).toBeGreaterThan(1);
  });

  it('SearchHit matches the /api/v1/search JSON tags (snake_case)', () => {
    const hit: SearchHit = {
      hash: '1_a1b2c3d4',
      short_hash: 'a1b2c3d',
      author: 'Alice',
      message: 'Fix the bug',
      repo_id: '1',
      tag: '',
    };
    expect(hit.repo_id).toBe('1');
  });

  it('DependencyLink matches the worker JSON tags (snake_case)', () => {
    // Compile-time contract check: this literal must satisfy DependencyLink
    // exactly as the Go/Rust worker serializes it for /api/v1/dependency-links.
    const link: DependencyLink = {
      from_repo: '1',
      to_repo: '2',
      via: 'github.com/acme/shared',
      kind: 'go',
    };
    expect(link.from_repo).toBe('1');
    expect(link.to_repo).toBe('2');
    expect(link.kind).toBe('go');

    const npmLink: DependencyLink = {
      from_repo: '3',
      to_repo: '1',
      via: '@acme/shared',
      kind: 'npm',
    };
    expect(npmLink.kind).toBe('npm');
  });

  it('AuthResponse carries token and role', () => {
    const res: AuthResponse = { access_token: 'jwt', role: 'Team Owner' };
    expect(res.role).toBe('Team Owner');
  });
});
