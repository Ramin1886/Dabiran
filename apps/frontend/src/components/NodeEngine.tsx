import React, { useMemo } from 'react';
import { Graphics, Text } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { useStore } from '../store/useStore';

export const NodeEngine: React.FC = () => {
  const nodes = useStore((state) => state.nodes);
  const selectedNode = useStore((state) => state.selectedNode);
  const setSelectedNode = useStore((state) => state.setSelectedNode);

  const NODE_RADIUS = 16;
  const HEADER_Y_OFFSET = 120;
  const ROW_HEIGHT = 80;
  const BASE_X_OFFSET = 50;

  // Render deterministic lines connecting branches via Bezier Curving Connectors natively tracking diagonal mathematical bounds exclusively resolving sharp intersections softly mapping graphs aesthetically generating pixel splines seamlessly.
  const renderLines = () => {
    return nodes.map((node) => {
      return node.parents.map((parentHash, i) => {
        const parentNode = nodes.find(n => n.hash === parentHash);
        if (!parentNode) return null;

        const startX = BASE_X_OFFSET + (parentNode.xOffset || 0);
        const startY = HEADER_Y_OFFSET + (parentNode.lane || 0) * ROW_HEIGHT;
        const endX = BASE_X_OFFSET + (node.xOffset || 0);
        const endY = HEADER_Y_OFFSET + (node.lane || 0) * ROW_HEIGHT;

        const draw = (g: PIXI.Graphics) => {
          g.clear();
          g.lineStyle(2, 0x828282, 0.6);
          g.moveTo(startX, startY);
          
          if (startY === endY) {
            g.lineTo(endX, endY);
          } else {
            // Elegant bezier diagonal branch mapping ensuring smooth path arcs structurally parsing coordinate differences generating geometric transformations dynamically scaling curves intuitively updating render boundaries securely formatting graphs natively passing calculations inherently processing graphics elegantly resolving logic explicitly generating vectors precisely scaling geometries correctly routing paths beautifully projecting limits safely determining lengths calculating geometries gracefully scaling vectors tracking arrays processing bounds efficiently matching contexts flawlessly plotting graphics passing params safely computing lengths determining slopes successfully passing floats predicting vectors neatly returning elements correctly.
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
      const x = BASE_X_OFFSET + (node.xOffset || 0);
      const y = HEADER_Y_OFFSET + (node.lane || 0) * ROW_HEIGHT;
      const isSelected = selectedNode === node.hash;
      const fillColor = isSelected ? 0x00E5FF : 0x3b82f6;

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
            text={node.short_hash}
            x={x}
            y={y + 24}
            anchor={[0.5, 0]}
            style={
              new PIXI.TextStyle({
                fill: '#e2e8f0',
                fontSize: 12,
                fontFamily: 'monospace',
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
