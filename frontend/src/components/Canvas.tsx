import React, { useEffect, useRef } from 'react';
import { Stage, Container } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { NodeEngine } from './NodeEngine';
import { useStore } from '../store/useStore';
import { MOCK_COMMITS } from '../mockData';

export const InteractiveCanvas: React.FC = () => {
  const setNodes = useStore((state) => state.setNodes);
  const containerRef = useRef<PIXI.Container>(null);

  // Initialize nodes into Zustand State (Simulating an API backend load)
  useEffect(() => {
    setNodes(MOCK_COMMITS);
  }, [setNodes]);

  const handlePointerDown = (e: PIXI.FederatedPointerEvent) => {
    // Zoom/Pan baseline dragging interaction logic could hook here
  };

  return (
    <div style={{ width: '100%', height: '100vh', overflow: 'hidden', position: 'relative' }}>
      <Stage 
        width={window.innerWidth} 
        height={window.innerHeight} 
        options={{ backgroundColor: 0x121212, antialias: true, autoDensity: true, resolution: window.devicePixelRatio }}
      >
        <Container 
          ref={containerRef}
          interactive={true}
          pointerdown={handlePointerDown}
        >
          {/* Main Visual Engine Sub-Layer */}
          <NodeEngine />
        </Container>
      </Stage>
    </div>
  );
};
