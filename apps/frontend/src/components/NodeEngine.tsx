import React, { useMemo } from 'react';
import { Graphics, Text } from '@pixi/react';
import * as PIXI from 'pixi.js';
import type { CommitNode } from '@git-viz/shared-types';
import { useStore } from '../store/useStore';

const NODE_RADIUS = 16;
const HEADER_Y_OFFSET = 120;
const ROW_HEIGHT = 80;
const BASE_X_OFFSET = 50;

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
 * NodeEngine renders the filtered commit topology (visibleNodes — the
 * search-aware subset, never the raw node list) as WebGL circles connected
 * by Bezier branch splines, using the backend layout fields x_offset/lane.
 */
export const NodeEngine: React.FC = () => {
  const visibleNodes = useStore((state) => state.visibleNodes);
  const selectedNode = useStore((state) => state.selectedNode);
  const setSelectedNode = useStore((state) => state.setSelectedNode);

  /** O(1) parent lookups while drawing connectors. */
  const nodesByHash = useMemo(
    () => new Map(visibleNodes.map((n) => [n.hash, n])),
    [visibleNodes],
  );

  /**
   * Renders the branch connector lines: straight segments along a lane and
   * Bezier diagonals across lanes for splits/merges.
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

  /** Renders the commit circles plus their tag/short-hash labels. */
  const renderNodes = () => {
    return visibleNodes.map((node) => {
      const x = nodeX(node);
      const y = nodeY(node);
      const isSelected = selectedNode === node.hash;
      const fillColor = isSelected ? 0x00e5ff : 0x3b82f6;

      const drawNode = (g: PIXI.Graphics) => {
        g.clear();
        g.lineStyle(isSelected ? 3 : 2, 0xffffff, 1);
        g.beginFill(fillColor, 1);
        g.drawCircle(x, y, NODE_RADIUS);
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
        </React.Fragment>
      );
    });
  };

  return (
    <>
      {renderLines()}
      {renderNodes()}
    </>
  );
};
