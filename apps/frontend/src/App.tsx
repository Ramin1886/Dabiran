import React from 'react';
import { InteractiveCanvas } from './components/Canvas';
import { useStore } from './store/useStore';

export default function App() {
  const selectedNode = useStore(state => state.selectedNode);
  const nodes = useStore(state => state.nodes);
  const activeNode = nodes.find(n => n.hash === selectedNode);

  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh', backgroundColor: '#000' }}>
      <div style={{ position: 'absolute', top: 0, left: 0, right: 0, padding: '20px 40px', background: 'rgba(255, 255, 255, 0.03)', backdropFilter: 'blur(10px)', zIndex: 10 }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#e2e8f0', letterSpacing: '-0.02em' }}>Git VR <span style={{ color: '#00E5FF' }}>Interactive</span></h1>
      </div>
      <InteractiveCanvas />
    </div>
  );
}
