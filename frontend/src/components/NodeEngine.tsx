import React, { useMemo } from 'react';
import { Graphics, Text } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { useStore, CommitNode } from '../store/useStore';

export const NodeEngine: React.FC = () => {
  const nodes = useStore((state) => state.nodes);
  const selectedNode = useStore((state) => state.selectedNode);
  const setSelectedNode = useStore((state) => state.setSelectedNode);

  // Configuration thresholds for aesthetic WebGL culling map layout
  const ROW_HEIGHT = 80;
  const NODE_RADIUS = 16;
  const HEADER_Y_OFFSET = 100; // base y start
  
  // Render deterministic lines connecting branches (Merges / Splits)
  const renderLines = () => {
    return nodes.map((node) => {
      return node.parents.map((parentHash, i) => {
        const parentNode = nodes.find(n => n.hash === parentHash);
        if (!parentNode) return null;

        const startX = parentNode.xOffset || 0;
        const startY = HEADER_Y_OFFSET + (parentNode.lane || 0) * ROW_HEIGHT;
        const endX = node.xOffset || 0;
        const endY = HEADER_Y_OFFSET + (node.lane || 0) * ROW_HEIGHT;

        const draw = (g: PIXI.Graphics) => {
          g.clear();
          g.lineStyle(2, 0x828282, 0.8);
          g.moveTo(startX, startY);
          
          if (startY === endY) {
            g.lineTo(endX, endY);
          } else {
            // Elegant bezier diagonal branch mapping
            const controlPointX = startX + (endX - startX) / 2;
            g.bezierCurveTo(controlPointX, startY, controlPointX, endY, endX, endY);
          }
        };

        return <Graphics key={`${parentHash}-${node.hash}-${i}`} draw={draw} />;
      });
    });
  };

  const renderNodes = () => {
    return nodes.map((node) => {
      const x = node.xOffset || 0;
      const y = HEADER_Y_OFFSET + (node.lane || 0) * ROW_HEIGHT;
      const isSelected = selectedNode === node.hash;
      const fillColor = isSelected ? 0x00E5FF : 0x3b82f6;

      const drawNode = (g: PIXI.Graphics) => {
        g.clear();
        g.lineStyle(2, 0xffffff, 1);
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
          {/* Node Metadata Label Priority */}
          <Text
            text={node.short_hash}
            x={x}
            y={y + 24}
            anchor={[0.5, 0]}
            style={
              new PIXI.TextStyle({
                fill: '#ffffff',
                fontSize: 12,
                fontFamily: 'Inter',
                fontWeight: '600'
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
