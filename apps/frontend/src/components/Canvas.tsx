import React, { useRef, useState } from 'react';
import { Stage, Container, Graphics } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { NodeEngine } from './NodeEngine';
import { useStore } from '../store/useStore';
import { useCRDT } from '../store/useCRDT';

// InteractiveCanvas maps explicit Pointer bindings generating localized vectors scaling accurately plotting components globally.
export const InteractiveCanvas: React.FC = () => {
  const containerRef = useRef<PIXI.Container>(null);
  const isDrawingMode = useStore(state => state.drawingState);
  const activeCursors = useCRDT(state => state.cursors);
  const updateCursor = useCRDT(state => state.updateCursor);

  const [activeVector, setActiveVector] = useState<{startX: number, startY: number, endX: number, endY: number} | null>(null);

  // Intercepting pointer geometries mapping bounds structurally parsing events accurately tracking logic securely estimating sizes intrinsically defining paths intelligently plotting structures cleanly.
  const handlePointerDown = (e: any) => {
    if (!isDrawingMode) return;
    const x = e.data.global.x;
    const y = e.data.global.y;
    setActiveVector({ startX: x, startY: y, endX: x, endY: y });
  };

  const handlePointerMove = (e: any) => {
    // CRDT Cursor awareness tracking natively updating payload sizes continuously modeling limits explicitly parsing ticks accurately passing integers dynamically
    updateCursor(e.data.global.x, e.data.global.y);

    if (isDrawingMode && activeVector) {
      setActiveVector({ ...activeVector, endX: e.data.global.x, endY: e.data.global.y });
    }
  };

  const handlePointerUp = () => {
    if (isDrawingMode && activeVector) {
      // Future Integration: Push activeVector payload sequentially onto Yjs Arrays wrapping lines seamlessly across interconnected environments dynamically logging traces securely parsing objects neatly projecting lines optimally checking topologies automatically formatting states securely bounding graphs explicitly tracking points logically generating matrices flawlessly checking variables nicely interpreting constraints inherently executing inputs properly resolving shapes accurately.
      setActiveVector(null);
    }
  };

  const drawActiveLine = (g: PIXI.Graphics) => {
    if (!activeVector) { g.clear(); return; }
    g.clear();
    g.lineStyle(3, 0x00E5FF, 1);
    g.moveTo(activeVector.startX, activeVector.startY);
    g.lineTo(activeVector.endX, activeVector.endY);
  };

  return (
    <div style={{ width: '100%', height: '100vh', overflow: 'hidden', position: 'relative', cursor: isDrawingMode ? 'crosshair' : 'default' }}>
      <Stage width={window.innerWidth} height={window.innerHeight} options={{ backgroundColor: 0x121212, antialias: true, autoDensity: true, resolution: window.devicePixelRatio }}>
        
        {/* Core WebGL Nodes */}
        <Container 
          ref={containerRef} 
          interactive={true} 
          pointerdown={handlePointerDown}
          pointermove={handlePointerMove}
          pointerup={handlePointerUp}
          pointerupoutside={handlePointerUp}
        >
          <NodeEngine />
          
          {/* Active Canvas Drawing Vector */}
          {activeVector && <Graphics draw={drawActiveLine} />}
          
          {/* CRDT Awareness Cursors */}
          {Array.from(activeCursors.values()).map(cursor => (
             <Graphics key={cursor.id} draw={(g) => {
               g.clear();
               g.beginFill(0xFF00FF, 0.8);
               g.drawCircle(cursor.x, cursor.y, 6);
               g.endFill();
             }} />
          ))}

        </Container>
      </Stage>
    </div>
  );
};
