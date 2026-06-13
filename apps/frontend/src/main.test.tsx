import { describe, it, expect, vi } from 'vitest';

const { renderMock, createRootMock } = vi.hoisted(() => {
  const renderMock = vi.fn();
  return {
    renderMock,
    createRootMock: vi.fn(() => ({ render: renderMock })),
  };
});

vi.mock('react-dom/client', () => ({ createRoot: createRootMock }));
vi.mock('./App', () => ({ default: () => null }));

describe('main entrypoint', () => {
  it('mounts <App/> onto #root via the React 18 createRoot API', async () => {
    document.body.innerHTML = '<div id="root"></div>';
    await import('./main');

    expect(createRootMock).toHaveBeenCalledTimes(1);
    expect(createRootMock).toHaveBeenCalledWith(document.getElementById('root'));
    expect(renderMock).toHaveBeenCalledTimes(1);
  });

  it('throws a descriptive error when the #root element is missing', async () => {
    const { mountApp } = await import('./main');
    document.body.innerHTML = '';
    expect(() => mountApp()).toThrow(/#root/);
  });
});
