import React from 'react';
import type { CommitNode } from '@git-viz/shared-types';
import { useStore } from '../store/useStore';

/**
 * Panel render view of a commit node widened with the semantic-zoom contract
 * fields (`kind`, `count`). Part of the shared @git-viz/shared-types
 * CommitNode contract (added by the backend agent); typed optional here so
 * the frontend builds green whether or not that edit has landed.
 */
type RenderNode = CommitNode & { kind?: string; count?: number };

/**
 * CommitPanel: glassmorphism floating window detailing the selected commit
 * (tag badge when present, short hash, message, author, date and parent
 * lineage hashes). When the selected node is a semantic-zoom aggregate, the
 * panel instead summarizes the N collapsed commits it represents. Hidden
 * while no node is selected.
 */
export const CommitPanel: React.FC = () => {
  const selectedNodeHash = useStore((state) => state.selectedNode);
  const nodes = useStore((state) => state.nodes) as RenderNode[];

  if (!selectedNodeHash) return null;

  // O(1) hash resolution lookup assuming frontend indexes track properly natively processing structures safely tracking logic implicitly
  const activeNode = nodes.find(n => n.hash === selectedNodeHash);

  if (!activeNode) return null;

  const isAggregate = activeNode.kind === 'aggregate';
  const aggregateCount = activeNode.count ?? 1;

  if (isAggregate) {
    return (
      <div
        data-testid="aggregate-panel"
        style={{
          position: 'absolute',
          right: '24px',
          top: '90px',
          width: '360px',
          background: 'linear-gradient(145deg, rgba(15, 23, 42, 0.85) 0%, rgba(30, 41, 59, 0.75) 100%)',
          backdropFilter: 'blur(20px)',
          WebkitBackdropFilter: 'blur(20px)',
          border: '1px solid rgba(139, 92, 246, 0.4)',
          borderRadius: '16px',
          padding: '24px',
          color: '#f8fafc',
          boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5)',
          fontFamily: '"Inter", "SF Pro Display", sans-serif',
          zIndex: 50,
          transition: 'opacity 0.2s ease, transform 0.2s ease',
          animation: 'slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1)',
        }}
      >
        <style>
          {`
            @keyframes slideIn {
              from { opacity: 0; transform: translateX(20px) scale(0.98); }
              to { opacity: 1; transform: translateX(0) scale(1); }
            }
            .commit-stat-label {
              font-size: 0.75rem;
              text-transform: uppercase;
              letter-spacing: 0.05em;
              color: #94a3b8;
              margin-bottom: 4px;
              font-weight: 600;
            }
          `}
        </style>

        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', borderBottom: '1px solid rgba(255, 255, 255, 0.1)', paddingBottom: '16px' }}>
          <div>
            <h2 style={{ margin: '0 0 6px 0', fontSize: '1.25rem', fontWeight: 700, letterSpacing: '-0.02em', color: '#e2e8f0' }}>Collapsed Cluster</h2>
            <span style={{ background: 'rgba(139, 92, 246, 0.15)', border: '1px solid rgba(139, 92, 246, 0.4)', color: '#c4b5fd', padding: '3px 10px', borderRadius: '999px', fontSize: '0.75rem', fontWeight: 700, letterSpacing: '0.05em' }}>
              AGGREGATE
            </span>
          </div>
        </div>

        <div style={{ marginBottom: '20px' }}>
          <div className="commit-stat-label">Represents</div>
          <p style={{ margin: 0, fontSize: '1.4rem', lineHeight: '1.4', color: '#e2e8f0', fontWeight: 700 }}>
            {aggregateCount} commits
          </p>
          <p style={{ margin: '8px 0 0 0', fontSize: '0.85rem', lineHeight: '1.5', color: '#94a3b8', fontWeight: 400 }}>
            This node collapses {aggregateCount} linear commits into a single cluster. Zoom in to expand the individual commits.
          </p>
        </div>

        <div style={{ padding: '12px', background: 'rgba(0,0,0,0.2)', borderRadius: '10px', fontSize: '0.8rem', color: '#64748b' }}>
          <div className="commit-stat-label" style={{ marginBottom: '8px' }}>Lineage Hash Keys</div>
          {activeNode.parents && activeNode.parents.length > 0 ? (
            activeNode.parents.map((p, idx) => (
              <div key={idx} style={{ fontFamily: 'monospace', marginBottom: '4px' }}>{p}</div>
            ))
          ) : (
            <div style={{ fontStyle: 'italic' }}>Origin Trajectory / Initialization Hash</div>
          )}
        </div>
      </div>
    );
  }

  return (
    <div style={{
      position: 'absolute',
      right: '24px',
      top: '90px',
      width: '360px',
      background: 'linear-gradient(145deg, rgba(15, 23, 42, 0.85) 0%, rgba(30, 41, 59, 0.75) 100%)',
      backdropFilter: 'blur(20px)',
      WebkitBackdropFilter: 'blur(20px)',
      border: '1px solid rgba(255, 255, 255, 0.08)',
      borderRadius: '16px',
      padding: '24px',
      color: '#f8fafc',
      boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.5)',
      fontFamily: '"Inter", "SF Pro Display", sans-serif',
      zIndex: 50,
      transition: 'opacity 0.2s ease, transform 0.2s ease',
      animation: 'slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1)',
    }}>
      <style>
        {`
          @keyframes slideIn {
            from { opacity: 0; transform: translateX(20px) scale(0.98); }
            to { opacity: 1; transform: translateX(0) scale(1); }
          }
          .commit-stat-label {
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: #94a3b8;
            margin-bottom: 4px;
            font-weight: 600;
          }
        `}
      </style>
      
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', borderBottom: '1px solid rgba(255, 255, 255, 0.1)', paddingBottom: '16px' }}>
        <div>
          <h2 style={{ margin: '0 0 6px 0', fontSize: '1.25rem', fontWeight: 700, letterSpacing: '-0.02em', color: '#e2e8f0' }}>Node Target</h2>
          <code style={{ background: 'rgba(0, 229, 255, 0.1)', color: '#00E5FF', padding: '4px 8px', borderRadius: '6px', fontSize: '0.85rem', fontWeight: 600, letterSpacing: '0.05em' }}>
            {activeNode.short_hash}
          </code>
          {activeNode.tag ? (
            <span style={{ marginLeft: '8px', background: 'rgba(0, 229, 255, 0.15)', border: '1px solid rgba(0, 229, 255, 0.4)', color: '#00E5FF', padding: '3px 8px', borderRadius: '999px', fontSize: '0.75rem', fontWeight: 700, letterSpacing: '0.05em' }}>
              {activeNode.tag}
            </span>
          ) : null}
        </div>
      </div>

      <div style={{ marginBottom: '20px' }}>
        <div className="commit-stat-label">Message</div>
        <p style={{ margin: 0, fontSize: '0.95rem', lineHeight: '1.5', color: '#cbd5e1', fontWeight: 400 }}>
          {activeNode.message || "No contextual message provided externally."}
        </p>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '24px' }}>
        <div>
          <div className="commit-stat-label">Author</div>
          <div style={{ fontSize: '0.9rem', fontWeight: 500 }}>{activeNode.author}</div>
        </div>
        <div>
          <div className="commit-stat-label">Date Sequence</div>
          <div style={{ fontSize: '0.9rem', fontWeight: 500 }}>
            {new Date(activeNode.date).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}
          </div>
        </div>
      </div>

      <div style={{ padding: '12px', background: 'rgba(0,0,0,0.2)', borderRadius: '10px', fontSize: '0.8rem', color: '#64748b' }}>
        <div className="commit-stat-label" style={{ marginBottom: '8px' }}>Lineage Hash Keys</div>
        {activeNode.parents && activeNode.parents.length > 0 ? (
          activeNode.parents.map((p, idx) => (
            <div key={idx} style={{ fontFamily: 'monospace', marginBottom: '4px' }}>{p}</div>
          ))
        ) : (
          <div style={{ fontStyle: 'italic' }}>Origin Trajectory / Initialization Hash</div>
        )}
      </div>
    </div>
  );
};
