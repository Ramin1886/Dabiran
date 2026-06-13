import React, { useRef, useState } from 'react';
import { Stage, Container, Graphics } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { screenToWorld, zoomAt } from '@git-viz/utils';
import { NodeEngine } from './NodeEngine';
import { useStore } from '../store/useStore';
import { useCRDT } from '../store/useCRDT';

/** Multiplicative zoom step applied per wheel tick. */
const ZOOM_STEP = 1.1;
/** Accent color used for locally drawn annotation vectors. */
const ACCENT_HEX = '#00E5FF';

/**
 * Converts a CSS hex color string (e.g. `#00E5FF`) into the numeric color
 * PixiJS v7 Graphics calls expect. Falls back to white on malformed input.
 *
 * @param hex - CSS hex color string
 * @returns numeric 0xRRGGBB color
 */
export function hexToNumber(hex: string): number {
  const parsed = parseInt(hex.replace('#', ''), 16);
  return Number.isNaN(parsed) ? 0xffffff : parsed;
}

/** Local world-space line segment being drawn before it is committed to CRDT. */
interface ActiveVector {
  startX: number;
  startY: number;
  endX: number;
  endY: number;
}

/**
 * InteractiveCanvas: the infinite pan/zoom WebGL viewport.
 *
 * - Wheel zooms anchored at the pointer (zoomAt from @git-viz/utils).
 * - Pointer drag on the canvas pans the world container (disabled while in
 *   drawing mode, where drags create annotation vectors instead).
 * - All pointer positions are converted to world space via screenToWorld
 *   before being used for drawing vectors and CRDT cursor broadcasts, so
 *   collaborators see positions anchored to the graph, not the screen.
 * - Persisted annotations from the shared Y.Array and remote awareness
 *   cursors render inside the world container at world coordinates.
 */
export const InteractiveCanvas: React.FC = () => {
  const viewportRef = useRef<HTMLDivElement>(null);
  const isDrawingMode = useStore((state) => state.drawingState);
  const transform = useStore((state) => state.viewportTransform);
  const setViewportTransform = useStore((state) => state.setViewportTransform);
  const activeCursors = useCRDT((state) => state.cursors);
  const annotations = useCRDT((state) => state.annotations);
  const updateCursor = useCRDT((state) => state.updateCursor);
  const addAnnotation = useCRDT((state) => state.addAnnotation);
  const ydoc = useCRDT((state) => state.ydoc);

  const [activeVector, setActiveVector] = useState<ActiveVector | null>(null);
  const panOrigin = useRef<{ x: number; y: number } | null>(null);

  /**
   * Translates a React pointer/wheel event into screen coordinates relative
   * to the canvas viewport element.
   */
  const toScreen = (e: { clientX: number; clientY: number }) => {
    const rect = viewportRef.current?.getBoundingClientRect();
    return { x: e.clientX - (rect?.left ?? 0), y: e.clientY - (rect?.top ?? 0) };
  };

  /** Wheel handler: zoom anchored at the pointer position. */
  const handleWheel = (e: React.WheelEvent<HTMLDivElement>) => {
    const p = toScreen(e);
    const factor = e.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP;
    setViewportTransform(zoomAt(transform, p.x, p.y, factor));
  };

  /**
   * Pointer-down: starts an annotation vector in drawing mode, otherwise
   * begins a pan gesture on the empty canvas.
   */
  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    const p = toScreen(e);
    if (isDrawingMode) {
      const w = screenToWorld(p.x, p.y, transform);
      setActiveVector({ startX: w.x, startY: w.y, endX: w.x, endY: w.y });
    } else {
      panOrigin.current = p;
    }
  };

  /**
   * Pointer-move: broadcasts the world-space cursor through CRDT awareness,
   * extends the active drawing vector, or applies the pan delta.
   */
  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const p = toScreen(e);
    const w = screenToWorld(p.x, p.y, transform);

    // CRDT cursor awareness — always world-space so remote canvases anchor
    // the cursor to the graph regardless of their own viewport.
    updateCursor(w.x, w.y);

    if (isDrawingMode && activeVector) {
      setActiveVector({ ...activeVector, endX: w.x, endY: w.y });
    } else if (!isDrawingMode && panOrigin.current) {
      const dx = p.x - panOrigin.current.x;
      const dy = p.y - panOrigin.current.y;
      panOrigin.current = p;
      setViewportTransform({
        ...transform,
        x: transform.x + dx,
        y: transform.y + dy,
      });
    }
  };

  /**
   * Pointer-up: commits the completed drawing vector into the shared
   * `annotations` Y.Array (replicated to all collaborators) and ends pans.
   */
  const handlePointerUp = () => {
    if (isDrawingMode && activeVector) {
      addAnnotation({
        start_x: activeVector.startX,
        start_y: activeVector.startY,
        end_x: activeVector.endX,
        end_y: activeVector.endY,
        color: ACCENT_HEX,
        author_id: ydoc.clientID,
      });
      setActiveVector(null);
    }
    panOrigin.current = null;
  };

  /** Draws the in-progress local annotation line. */
  const drawActiveLine = (g: PIXI.Graphics) => {
    g.clear();
    if (!activeVector) return;
    g.lineStyle(3, hexToNumber(ACCENT_HEX), 1);
    g.moveTo(activeVector.startX, activeVector.startY);
    g.lineTo(activeVector.endX, activeVector.endY);
  };

  return (
    <div
      ref={viewportRef}
      data-testid="canvas-viewport"
      onWheel={handleWheel}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerLeave={handlePointerUp}
      style={{
        width: '100%',
        height: '100vh',
        overflow: 'hidden',
        position: 'relative',
        cursor: isDrawingMode ? 'crosshair' : 'grab',
        touchAction: 'none',
      }}
    >
      <Stage
        width={window.innerWidth}
        height={window.innerHeight}
        options={{
          backgroundColor: 0x0f172a,
          antialias: true,
          autoDensity: true,
          resolution: window.devicePixelRatio,
        }}
      >
        {/* World container — the infinite canvas. The persisted viewport
            transform maps world space onto the screen. */}
        <Container x={transform.x} y={transform.y} scale={transform.scale}>
          <NodeEngine />

          {/* Persisted collaborative annotation vectors (shared Y.Array). */}
          {annotations.map((a, i) => (
            <Graphics
              key={`annotation-${i}`}
              draw={(g: PIXI.Graphics) => {
                g.clear();
                g.lineStyle(3, hexToNumber(a.color), 1);
                g.moveTo(a.start_x, a.start_y);
                g.lineTo(a.end_x, a.end_y);
              }}
            />
          ))}

          {/* Active local drawing vector (world space). */}
          {activeVector && <Graphics draw={drawActiveLine} />}

          {/* CRDT awareness cursors rendered at world coordinates. */}
          {Array.from(activeCursors.values()).map((cursor) => (
            <Graphics
              key={cursor.id}
              draw={(g: PIXI.Graphics) => {
                g.clear();
                g.beginFill(hexToNumber(cursor.color), 0.8);
                g.drawCircle(cursor.x, cursor.y, 6);
                g.endFill();
              }}
            />
          ))}
        </Container>
      </Stage>
    </div>
  );
};
