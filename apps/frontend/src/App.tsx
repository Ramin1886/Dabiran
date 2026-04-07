import React from 'react';
import { InteractiveCanvas } from './components/Canvas';
import { CommitPanel } from './components/CommitPanel';

// Main App binds absolute topological components mapping WebGL engines underneath Native DOM interactions tracking logic natively securely.
export default function App() {
  return (
    <div style={{ position: 'relative', width: '100vw', height: '100vh', backgroundColor: '#0f172a', overflow: 'hidden' }}>
      
      {/* HUD Layer resolving Navigation/Logo bounds isolated completely from rendering loops naturally. */}
      <div style={{ 
        position: 'absolute', 
        top: 0, 
        left: 0, 
        right: 0, 
        padding: '20px 40px', 
        background: 'linear-gradient(to bottom, rgba(15, 23, 42, 0.9) 0%, rgba(15, 23, 42, 0) 100%)', 
        zIndex: 10,
        pointerEvents: 'none',
      }}>
        <h1 style={{ 
          margin: 0, 
          fontSize: '1.5rem', 
          color: '#e2e8f0', 
          letterSpacing: '-0.02em',
          fontFamily: '"Inter", sans-serif',
          fontWeight: 800,
        }}>
          Git VR <span style={{ color: '#00E5FF' }}>Interactive</span>
        </h1>
      </div>

      {/* WebGL Canvas Pipeline */}
      <InteractiveCanvas />

      {/* Contextual Interaction Floating Windows bounding logic accurately mapped against Z index bounds */}
      <CommitPanel />
      
    </div>
  );
}
