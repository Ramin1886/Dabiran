import React from 'react';
import { InteractiveCanvas } from './components/Canvas';
import { useStore } from './store/useStore';

export default function App() {
  const selectedNode = useStore(state => state.selectedNode);
  const nodes = useStore(state => state.nodes);

  const activeNode = nodes.find(n => n.hash === selectedNode);

  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh', backgroundColor: '#000' }}>
      {/* Absolute top header Glassmorphism style */}
      <div style={{
          position: 'absolute', top: 0, left: 0, right: 0, padding: '20px 40px',
          background: 'rgba(255, 255, 255, 0.03)',
          backdropFilter: 'blur(10px)',
          borderBottom: '1px solid rgba(255, 255, 255, 0.1)',
          display: 'flex', justifyContent: 'space-between', alignItems: 'center', zIndex: 10
      }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 600, color: '#e2e8f0', letterSpacing: '-0.02em' }}>
          Git VR <span style={{ color: '#00E5FF', marginLeft: 8 }}>Interactive Canvas</span>
        </h1>
        <div style={{ color: '#94a3b8', fontSize: '0.875rem' }}>
          View: Continuous Pipeline
        </div>
      </div>

      <InteractiveCanvas />

      {/* Conditional Right Context Panel */}
      {activeNode && (
        <div style={{
            position: 'absolute', top: 90, right: 20, width: '320px',
            background: 'rgba(20, 20, 20, 0.8)', backdropFilter: 'blur(16px)',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            borderRadius: '12px', padding: '24px',
            color: '#f8fafc', zIndex: 10,
            boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5)'
        }}>
          <h3 style={{ margin: '0 0 16px 0', fontSize: '1rem', color: '#00E5FF' }}>Commit Details</h3>
          <p style={{ margin: '8px 0', fontSize: '0.875rem', color: '#94a3b8' }}>Hash:<br/><span style={{fontFamily: 'monospace', color: '#fff'}}>{activeNode.hash}</span></p>
          <p style={{ margin: '8px 0', fontSize: '0.875rem', color: '#94a3b8' }}>Author: <span style={{color: '#fff'}}>{activeNode.author}</span></p>
          <p style={{ margin: '8px 0', fontSize: '0.875rem', color: '#94a3b8' }}>Date: {new Date(activeNode.date).toLocaleString()}</p>
          <div style={{ marginTop: '16px', padding: '12px', background: 'rgba(255, 255, 255, 0.05)', borderRadius: '6px' }}>
            {activeNode.message}
          </div>
        </div>
      )}
    </div>
  );
}
