import React from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';

/**
 * Application entrypoint: mounts the root <App/> component onto the #root
 * element declared in index.html using the React 18 concurrent root API.
 *
 * @throws Error when the #root mount element is missing from the document.
 */
export function mountApp(): void {
  const rootElement = document.getElementById('root');
  if (!rootElement) {
    throw new Error('Root element #root not found — index.html is malformed.');
  }
  createRoot(rootElement).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  );
}

mountApp();
