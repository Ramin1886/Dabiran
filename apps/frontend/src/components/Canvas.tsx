import React, { useEffect, useRef } from 'react';
import { Stage, Container } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { NodeEngine } from './NodeEngine';
import { useStore } from '../store/useStore';

export const InteractiveCanvas: React.FC = () => {
  const containerRef = useRef<PIXI.Container>(null);

  return (
    <div style={{ width: '100%', height: '100vh', overflow: 'hidden', position: 'relative' }}>
      <Stage width={window.innerWidth} height={window.innerHeight} options={{ backgroundColor: 0x121212, antialias: true, autoDensity: true, resolution: window.devicePixelRatio }}>
        <Container ref={containerRef} interactive={true}>
          <NodeEngine />
        </Container>
      </Stage>
    </div>
  );
};
