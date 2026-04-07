import React, { useMemo } from 'react';
import { Graphics, Text } from '@pixi/react';
import * as PIXI from 'pixi.js';
import { useStore } from '../store/useStore';

export const NodeEngine: React.FC = () => {
  const nodes = useStore((state) => state.nodes);
  const selectedNode = useStore((state) => state.selectedNode);
  const setSelectedNode = useStore((state) => state.setSelectedNode);

  const renderNodes = () => nodes.map((node) => {
    const x = node.xOffset || 0;
    const y = 100 + (node.lane || 0) * 80;
    const draw = (g: PIXI.Graphics) => { g.clear(); g.lineStyle(2, 0xffffff, 1); g.beginFill(selectedNode === node.hash ? 0x00E5FF : 0x3b82f6, 1); g.drawCircle(x, y, 16); g.endFill(); };
    return <React.Fragment key={node.hash}><Graphics draw={draw} interactive={true} pointerdown={() => setSelectedNode(node.hash)} cursor="pointer"/><Text text={node.short_hash} x={x} y={y + 24} anchor={[0.5, 0]} style={new PIXI.TextStyle({ fill: '#ffffff', fontSize: 12, fontWeight: '600' })}/></React.Fragment>;
  });

  return <>{renderNodes()}</>;
};
