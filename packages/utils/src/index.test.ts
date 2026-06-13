import { describe, expect, it } from 'vitest';
import {
  makePrefixedHash,
  parsePrefixedHash,
  screenToWorld,
  zoomAt,
} from './index';

describe('prefixed hash codec', () => {
  it('round-trips repo id and sha', () => {
    const id = makePrefixedHash(7, 'a1b2c3d4');
    expect(id).toBe('7_a1b2c3d4');
    expect(parsePrefixedHash(id)).toEqual({ repoId: '7', sha: 'a1b2c3d4' });
  });

  it('splits at the first underscore only', () => {
    // Defensive: SHAs never contain underscores, but repo ids stay intact.
    expect(parsePrefixedHash('12_abc')).toEqual({ repoId: '12', sha: 'abc' });
  });

  it('returns null for malformed ids', () => {
    expect(parsePrefixedHash('nounderscore')).toBeNull();
    expect(parsePrefixedHash('_leading')).toBeNull();
    expect(parsePrefixedHash('trailing_')).toBeNull();
  });
});

describe('viewport math', () => {
  it('screenToWorld inverts the viewport transform', () => {
    const t = { x: 100, y: 50, scale: 2 };
    expect(screenToWorld(300, 250, t)).toEqual({ x: 100, y: 100 });
  });

  it('zoomAt keeps the anchor point fixed in world space', () => {
    const t = { x: 0, y: 0, scale: 1 };
    const next = zoomAt(t, 400, 300, 2);
    const before = screenToWorld(400, 300, t);
    const after = screenToWorld(400, 300, next);
    expect(after.x).toBeCloseTo(before.x);
    expect(after.y).toBeCloseTo(before.y);
    expect(next.scale).toBe(2);
  });

  it('zoomAt clamps to min and max scale', () => {
    const t = { x: 0, y: 0, scale: 1 };
    expect(zoomAt(t, 0, 0, 1000).scale).toBe(8);
    expect(zoomAt(t, 0, 0, 0.000001).scale).toBe(0.05);
  });
});
