import React, { useEffect, useMemo, useState } from 'react';
import { Graphics, Text } from '@pixi/react';
import * as PIXI from 'pixi.js';
import type { CommitNode, DependencyLink } from '@git-viz/shared-types';
import { screenToWorld } from '@git-viz/utils';
import { useStore } from '../store/useStore';

const NODE_RADIUS = 16;
const HEADER_Y_OFFSET = 120;
const ROW_HEIGHT = 80;
const BASE_X_OFFSET = 50;
/** Half-width/height of the aggregate cluster rounded-rect glyph. */
const AGG_HALF_W = 28;
const AGG_HALF_H = 18;
/**
 * Amber accent for auto-generated cross-repo dependency links — visually
 * distinct from the cyan (#00E5FF) user-drawn annotation vectors and the grey
 * (0x828282) commit connectors.
 */
const DEPENDENCY_HEX = 0xf59e0b;
/** Dash/gap lengths (world units) for the emulated dashed dependency line. */
const DASH_LENGTH = 10;
const DASH_GAP = 6;
/** Max characters of the `via` label before it is truncated with an ellipsis. */
const VIA_LABEL_MAX = 24;

/**
 * Frontend render view of a commit node widened with the two semantic-zoom
 * fields from the SHARED CONTRACT (`kind` and `count`). They are part of the
 * @git-viz/shared-types CommitNode contract added by the backend agent, but
 * are typed optional here so the frontend builds green whether or not that
 * shared-types edit has landed yet.
 */
export type RenderNode = CommitNode & { kind?: string; count?: number };

/** Axis-aligned world-space rectangle used for viewport culling. */
interface WorldRect {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
}

/**
 * Resolves a node's world-space X coordinate from the backend-assigned
 * chronological `x_offset` (snake_case wire field).
 */
function nodeX(node: CommitNode): number {
  return BASE_X_OFFSET + (node.x_offset || 0);
}

/**
 * Resolves a node's world-space Y coordinate from the backend-assigned
 * branch `lane` (snake_case wire field).
 */
function nodeY(node: CommitNode): number {
  return HEADER_Y_OFFSET + (node.lane || 0) * ROW_HEIGHT;
}

/**
 * Resolves the canvas label for a commit. Label priority per the docs:
 * render the tag text when `node.tag` is non-empty, else the short hash.
 */
export function nodeLabel(node: CommitNode): string {
  return node.tag ? node.tag : node.short_hash;
}

/**
 * True when a node is a semantic-zoom aggregate cluster (kind 'aggregate'),
 * representing several collapsed linear commits. Part of the shared contract.
 */
export function isAggregate(node: RenderNode): boolean {
  return node.kind === 'aggregate';
}

/**
 * Resolves the cluster glyph label for an aggregate node, e.g. "+5". Falls
 * back to a count of 1 when the contract field is absent.
 */
export function aggregateLabel(node: RenderNode): string {
  return `+${node.count ?? 1}`;
}

/**
 * Tests whether a world point falls inside a rectangle (inclusive bounds).
 */
function pointInRect(x: number, y: number, r: WorldRect): boolean {
  return x >= r.minX && x <= r.maxX && y >= r.minY && y <= r.maxY;
}

/**
 * Tests whether a connector segment from (ax,ay) to (bx,by) is relevant to
 * the visible rect: relevant when either endpoint is inside, or when the
 * segment's bounding box overlaps the rect (cheap conservative crossing test
 * that keeps long diagonals spanning the viewport drawn).
 */
function segmentTouchesRect(
  ax: number,
  ay: number,
  bx: number,
  by: number,
  r: WorldRect,
): boolean {
  if (pointInRect(ax, ay, r) || pointInRect(bx, by, r)) return true;
  const segMinX = Math.min(ax, bx);
  const segMaxX = Math.max(ax, bx);
  const segMinY = Math.min(ay, by);
  const segMaxY = Math.max(ay, by);
  return (
    segMaxX >= r.minX &&
    segMinX <= r.maxX &&
    segMaxY >= r.minY &&
    segMinY <= r.maxY
  );
}

/**
 * Resolves the deterministic representative node for a repository: the latest
 * (maximum `x_offset`) node among `nodes` whose `repo_id` matches `repoId`
 * (compared as strings). Returns null when no node belongs to that repo, so the
 * caller can skip the dependency link.
 *
 * @param nodes - the candidate (visible) nodes to choose an endpoint from
 * @param repoId - the dependency link repo identifier (string)
 */
export function representativeNode(
  nodes: CommitNode[],
  repoId: string,
): CommitNode | null {
  let best: CommitNode | null = null;
  for (const node of nodes) {
    if (String(node.repo_id) !== String(repoId)) continue;
    if (best === null || (node.x_offset || 0) > (best.x_offset || 0)) {
      best = node;
    }
  }
  return best;
}

/**
 * Truncates a dependency `via` label to {@link VIA_LABEL_MAX} characters,
 * appending an ellipsis when shortened, so long module paths stay legible.
 */
export function truncateVia(via: string): string {
  return via.length > VIA_LABEL_MAX ? `${via.slice(0, VIA_LABEL_MAX - 1)}…` : via;
}

/**
 * Draws a dashed straight line from (ax,ay) to (bx,by) onto a PixiJS v7
 * Graphics. PixiJS v7 has no native dash for Graphics, so the segment is split
 * into alternating short dashes/gaps of {@link DASH_LENGTH}/{@link DASH_GAP}.
 */
function drawDashedLine(
  g: PIXI.Graphics,
  ax: number,
  ay: number,
  bx: number,
  by: number,
): void {
  const dx = bx - ax;
  const dy = by - ay;
  const length = Math.hypot(dx, dy);
  if (length === 0) return;
  const ux = dx / length;
  const uy = dy / length;
  const step = DASH_LENGTH + DASH_GAP;
  let travelled = 0;
  while (travelled < length) {
    const dashEnd = Math.min(travelled + DASH_LENGTH, length);
    g.moveTo(ax + ux * travelled, ay + uy * travelled);
    g.lineTo(ax + ux * dashEnd, ay + uy * dashEnd);
    travelled += step;
  }
}

/**
 * Reads the current canvas pixel size. Mirrors the Stage sizing in Canvas.tsx
 * (full window). Guards against a missing window for non-DOM test envs.
 */
function readScreenSize(): { width: number; height: number } {
  if (typeof window === 'undefined') return { width: 0, height: 0 };
  return { width: window.innerWidth, height: window.innerHeight };
}

/**
 * NodeEngine renders the filtered commit topology (visibleNodes — the
 * search-aware subset, never the raw node list) as WebGL circles (normal
 * commits) or rounded-rect cluster glyphs (semantic-zoom aggregates),
 * connected by Bezier branch splines, using the backend layout fields
 * x_offset/lane.
 *
 * Performance: only nodes inside the padded visible world rectangle are
 * rendered, and a connector is drawn only when its segment touches that rect.
 * The rect is derived with useMemo from the viewport transform + screen size,
 * and a window resize listener keeps the screen size fresh.
 */
export const NodeEngine: React.FC = () => {
  const visibleNodes = useStore((state) => state.visibleNodes) as RenderNode[];
  const selectedNode = useStore((state) => state.selectedNode);
  const setSelectedNode = useStore((state) => state.setSelectedNode);
  const transform = useStore((state) => state.viewportTransform);
  const dependencyLinks = useStore((state) => state.dependencyLinks);

  // Track the screen size so the visible rect recomputes on window resize.
  const [screenSize, setScreenSize] = useState(readScreenSize);

  useEffect(() => {
    const onResize = () => setScreenSize(readScreenSize());
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  /**
   * The padded visible world-space rectangle. Computed by mapping the four
   * screen corners back to world space via screenToWorld, then padding by one
   * node row so nodes/labels never pop in/out exactly at the edges.
   */
  const visibleRect = useMemo<WorldRect>(() => {
    const topLeft = screenToWorld(0, 0, transform);
    const bottomRight = screenToWorld(
      screenSize.width,
      screenSize.height,
      transform,
    );
    const pad = ROW_HEIGHT;
    return {
      minX: topLeft.x - pad,
      minY: topLeft.y - pad,
      maxX: bottomRight.x + pad,
      maxY: bottomRight.y + pad,
    };
  }, [transform, screenSize.width, screenSize.height]);

  /** O(1) parent lookups while drawing connectors. */
  const nodesByHash = useMemo(
    () => new Map(visibleNodes.map((n) => [n.hash, n])),
    [visibleNodes],
  );

  /**
   * Renders the branch connector lines: straight segments along a lane and
   * Bezier diagonals across lanes for splits/merges. Culled to connectors
   * whose segment touches the visible rect.
   */
  const renderLines = () => {
    return visibleNodes.map((node) => {
      return node.parents.map((parentHash, i) => {
        const parentNode = nodesByHash.get(parentHash);
        if (!parentNode) return null;

        const startX = nodeX(parentNode);
        const startY = nodeY(parentNode);
        const endX = nodeX(node);
        const endY = nodeY(node);

        if (!segmentTouchesRect(startX, startY, endX, endY, visibleRect)) {
          return null;
        }

        const draw = (g: PIXI.Graphics) => {
          g.clear();
          g.lineStyle(2, 0x828282, 0.6);
          g.moveTo(startX, startY);

          if (startY === endY) {
            g.lineTo(endX, endY);
          } else {
            // Bezier diagonal mapping split/merge lane crossings smoothly.
            const controlPointX = startX + (endX - startX) / 2;
            g.bezierCurveTo(controlPointX, startY, controlPointX, endY, endX, endY);
          }
        };

        return <Graphics key={`${parentHash}-${node.hash}-${i}`} draw={draw} />;
      });
    });
  };

  /**
   * Renders the commit glyphs plus labels: normal commits as circles, and
   * aggregate (semantic-zoom) nodes as a distinct rounded-rect cluster with a
   * "+N" count label. Culled to nodes inside the visible rect.
   */
  const renderNodes = () => {
    return visibleNodes.map((node) => {
      const x = nodeX(node);
      const y = nodeY(node);

      if (!pointInRect(x, y, visibleRect)) return null;

      const isSelected = selectedNode === node.hash;
      const aggregate = isAggregate(node);
      const fillColor = isSelected ? 0x00e5ff : aggregate ? 0x8b5cf6 : 0x3b82f6;

      const drawNode = (g: PIXI.Graphics) => {
        g.clear();
        g.lineStyle(isSelected ? 3 : 2, 0xffffff, 1);
        g.beginFill(fillColor, 1);
        if (aggregate) {
          // Distinct cluster glyph: rounded rectangle (not a circle) so users
          // visually read it as a collapsed group of commits.
          g.drawRoundedRect(
            x - AGG_HALF_W,
            y - AGG_HALF_H,
            AGG_HALF_W * 2,
            AGG_HALF_H * 2,
            8,
          );
        } else {
          g.drawCircle(x, y, NODE_RADIUS);
        }
        g.endFill();
      };

      return (
        <React.Fragment key={node.hash}>
          <Graphics
            draw={drawNode}
            interactive={true}
            pointerdown={() => setSelectedNode(node.hash)}
            cursor="pointer"
          />
          {aggregate ? (
            <Text
              text={aggregateLabel(node)}
              x={x}
              y={y}
              anchor={[0.5, 0.5]}
              style={
                new PIXI.TextStyle({
                  fill: '#f8fafc',
                  fontSize: 13,
                  fontFamily: 'monospace',
                  fontWeight: '700',
                })
              }
            />
          ) : (
            <Text
              text={nodeLabel(node)}
              x={x}
              y={y + 24}
              anchor={[0.5, 0]}
              style={
                new PIXI.TextStyle({
                  fill: '#e2e8f0',
                  fontSize: 12,
                  fontFamily: 'monospace',
                  fontWeight: '600',
                })
              }
            />
          )}
        </React.Fragment>
      );
    });
  };

  /**
   * Renders the auto-generated cross-repository dependency links as distinct
   * dashed amber lines connecting a representative node of `from_repo` to one
   * of `to_repo`, labelled with the (truncated) `via` module path.
   *
   * Endpoints are chosen deterministically via {@link representativeNode} (the
   * latest/max-x_offset visible node of each repo). A link is skipped when
   * either repo has no visible node. Culled with the same viewport approach as
   * commit connectors: a link is dropped when its segment never touches the
   * visible rect.
   */
  const renderDependencyLinks = () => {
    return dependencyLinks.map((link: DependencyLink, i) => {
      const fromNode = representativeNode(visibleNodes, link.from_repo);
      const toNode = representativeNode(visibleNodes, link.to_repo);
      // Skip links whose source or target repo has no visible node.
      if (!fromNode || !toNode) return null;

      const startX = nodeX(fromNode);
      const startY = nodeY(fromNode);
      const endX = nodeX(toNode);
      const endY = nodeY(toNode);

      if (!segmentTouchesRect(startX, startY, endX, endY, visibleRect)) {
        return null;
      }

      const midX = (startX + endX) / 2;
      const midY = (startY + endY) / 2;
      const key = `dep-${link.from_repo}-${link.to_repo}-${i}`;

      const drawLink = (g: PIXI.Graphics) => {
        g.clear();
        g.lineStyle(2, DEPENDENCY_HEX, 0.9);
        drawDashedLine(g, startX, startY, endX, endY);
      };

      return (
        <React.Fragment key={key}>
          <Graphics draw={drawLink} />
          <Text
            text={truncateVia(link.via)}
            x={midX}
            y={midY - 10}
            anchor={[0.5, 1]}
            style={
              new PIXI.TextStyle({
                fill: '#f59e0b',
                fontSize: 11,
                fontFamily: 'monospace',
                fontWeight: '600',
              })
            }
          />
        </React.Fragment>
      );
    });
  };

  return (
    <>
      {renderLines()}
      {renderDependencyLinks()}
      {renderNodes()}
    </>
  );
};
