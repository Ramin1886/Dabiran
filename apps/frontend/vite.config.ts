/// <reference types="vitest" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

/**
 * Vite configuration for the WebGL canvas frontend.
 *
 * DEPENDENCY PIN NOTE: pixi.js is intentionally pinned to ^7.4.x together
 * with @pixi/react ^7.1.x. The component layer (Canvas/NodeEngine) is written
 * against the PixiJS v7 imperative Graphics API (g.lineStyle / g.beginFill /
 * g.drawCircle) and the @pixi/react v7 declarative components
 * (Stage/Container/Graphics/Text). PixiJS v8 removed/renamed these APIs and
 * @pixi/react v8 is a different component model, so do NOT bump pixi.js to v8
 * without rewriting the rendering layer.
 *
 * Dev server runs on port 3000 per docs/local-setup.md.
 */
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
});
