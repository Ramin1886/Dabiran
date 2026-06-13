/**
 * Shared utility helpers used by the frontend canvas and any future
 * TypeScript workers. Pure functions only — no DOM or network access — so
 * they stay trivially testable and reusable.
 */

/** Decoded form of a `<RepoID>_<SHA>` prefixed node identifier. */
export interface PrefixedHash {
  repoId: string;
  sha: string;
}

/**
 * Encodes a repository id and commit SHA into the collision-free node id
 * used across the unified multi-repo canvas (docs/apis_doc.md
 * "Hash Prefix Strategy").
 *
 * @param repoId - repository identifier (numeric or string)
 * @param sha - full commit SHA
 * @returns prefixed id, e.g. `1_a1b2c3d`
 */
export function makePrefixedHash(repoId: number | string, sha: string): string {
  return `${repoId}_${sha}`;
}

/**
 * Decodes a `<RepoID>_<SHA>` node id back into its parts. SHAs never contain
 * underscores, so the split happens at the first underscore only.
 *
 * @param prefixed - id of the form `1_a1b2c3d...`
 * @returns the decoded parts, or null when the input has no prefix separator
 */
export function parsePrefixedHash(prefixed: string): PrefixedHash | null {
  const idx = prefixed.indexOf('_');
  if (idx <= 0 || idx === prefixed.length - 1) return null;
  return { repoId: prefixed.slice(0, idx), sha: prefixed.slice(idx + 1) };
}

/** Viewport transform applied to the world-space canvas container. */
export interface ViewportTransform {
  x: number;
  y: number;
  scale: number;
}

/**
 * Converts a screen-space point (e.g. a pointer event) into world-space
 * canvas coordinates under the given viewport transform.
 *
 * @param screenX - pointer X in screen pixels
 * @param screenY - pointer Y in screen pixels
 * @param t - active viewport transform
 * @returns world-space coordinates
 */
export function screenToWorld(
  screenX: number,
  screenY: number,
  t: ViewportTransform,
): { x: number; y: number } {
  return { x: (screenX - t.x) / t.scale, y: (screenY - t.y) / t.scale };
}

/**
 * Computes the next viewport transform for a zoom gesture anchored at a
 * screen point, so the world point under the pointer stays fixed while
 * scaling (standard infinite-canvas zoom behavior).
 *
 * @param t - current viewport transform
 * @param anchorX - screen X of the zoom anchor (pointer position)
 * @param anchorY - screen Y of the zoom anchor
 * @param scaleFactor - multiplicative zoom step (>1 zooms in)
 * @param minScale - lower clamp for the resulting scale
 * @param maxScale - upper clamp for the resulting scale
 * @returns the new viewport transform
 */
export function zoomAt(
  t: ViewportTransform,
  anchorX: number,
  anchorY: number,
  scaleFactor: number,
  minScale = 0.05,
  maxScale = 8,
): ViewportTransform {
  const nextScale = Math.min(maxScale, Math.max(minScale, t.scale * scaleFactor));
  const ratio = nextScale / t.scale;
  return {
    scale: nextScale,
    x: anchorX - (anchorX - t.x) * ratio,
    y: anchorY - (anchorY - t.y) * ratio,
  };
}
