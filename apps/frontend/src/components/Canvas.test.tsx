import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/react';
import { InteractiveCanvas, hexToNumber } from './Canvas';
import { useStore } from '../store/useStore';
import { useCRDT } from '../store/useCRDT';

vi.mock('@pixi/react');
vi.mock('y-websocket');

/* eslint-disable @typescript-eslint/no-explicit-any */

// jsdom has no PointerEvent — MouseEvent carries the same coordinate payload
// and React's pointer handlers listen by event type, not constructor.
vi.stubGlobal('PointerEvent', MouseEvent);

/** Dispatches a pointer event with coordinates on the canvas viewport. */
function firePointer(el: Element, type: string, x: number, y: number) {
  fireEvent(
    el,
    new MouseEvent(type, { bubbles: true, cancelable: true, clientX: x, clientY: y }),
  );
}

describe('InteractiveCanvas', () => {
  beforeEach(() => {
    useStore.setState({
      nodes: [],
      visibleNodes: [],
      searchQuery: '',
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
    });
    // Fresh CRDT doc/provider per test (mocked websocket).
    useCRDT.getState().connect('repo_map_1');
  });

  it('hexToNumber parses accent colors and falls back to white', () => {
    expect(hexToNumber('#00E5FF')).toBe(0x00e5ff);
    expect(hexToNumber('garbage')).toBe(0xffffff);
  });

  it('renders the PixiJS stage smoke-free', () => {
    render(<InteractiveCanvas />);
    expect(screen.getByTestId('pixi-stage')).toBeTruthy();
    expect(screen.getByTestId('canvas-viewport')).toBeTruthy();
  });

  it('wheel zooms anchored at the pointer and persists the transform in the store', () => {
    render(<InteractiveCanvas />);
    const viewport = screen.getByTestId('canvas-viewport');

    fireEvent.wheel(viewport, { deltaY: -100, clientX: 200, clientY: 100 });

    const t = useStore.getState().viewportTransform;
    expect(t.scale).toBeCloseTo(1.1);
    // Anchor math: the world point under (200,100) stays fixed.
    expect(t.x).toBeCloseTo(200 - 200 * 1.1);
    expect(t.y).toBeCloseTo(100 - 100 * 1.1);

    fireEvent.wheel(viewport, { deltaY: 100, clientX: 200, clientY: 100 });
    expect(useStore.getState().viewportTransform.scale).toBeCloseTo(1);
  });

  it('pointer drag pans the viewport when not in drawing mode', () => {
    render(<InteractiveCanvas />);
    const viewport = screen.getByTestId('canvas-viewport');

    firePointer(viewport, 'pointerdown', 10, 10);
    firePointer(viewport, 'pointermove', 30, 25);
    firePointer(viewport, 'pointerup', 30, 25);

    const t = useStore.getState().viewportTransform;
    expect(t.x).toBe(20);
    expect(t.y).toBe(15);
    expect(t.scale).toBe(1);
  });

  it('does not pan while drawing mode is active; drag persists an annotation vector instead', () => {
    useStore.setState({ drawingState: true });
    render(<InteractiveCanvas />);
    const viewport = screen.getByTestId('canvas-viewport');

    firePointer(viewport, 'pointerdown', 5, 6);
    firePointer(viewport, 'pointermove', 50, 60);
    firePointer(viewport, 'pointerup', 50, 60);

    // Viewport untouched.
    expect(useStore.getState().viewportTransform).toEqual({ x: 0, y: 0, scale: 1 });

    // Completed vector landed in the shared annotations Y.Array.
    const annotations = useCRDT.getState().annotations;
    expect(annotations).toHaveLength(1);
    expect(annotations[0]).toMatchObject({ start_x: 5, start_y: 6, end_x: 50, end_y: 60 });
  });

  it('stores drawing vectors in world space under a panned/zoomed viewport', () => {
    useStore.setState({
      drawingState: true,
      viewportTransform: { x: 100, y: 50, scale: 2 },
    });
    render(<InteractiveCanvas />);
    const viewport = screen.getByTestId('canvas-viewport');

    firePointer(viewport, 'pointerdown', 100, 50); // world (0, 0)
    firePointer(viewport, 'pointermove', 140, 90); // world (20, 20)
    firePointer(viewport, 'pointerup', 140, 90);

    const annotations = useCRDT.getState().annotations;
    expect(annotations[0]).toMatchObject({ start_x: 0, start_y: 0, end_x: 20, end_y: 20 });
  });

  it('broadcasts CRDT cursor positions in world space, not screen space', () => {
    useStore.setState({ viewportTransform: { x: 100, y: 0, scale: 2 } });
    render(<InteractiveCanvas />);
    const viewport = screen.getByTestId('canvas-viewport');

    firePointer(viewport, 'pointermove', 140, 10);

    const provider = useCRDT.getState().provider as any;
    expect(provider.awareness.setLocalStateField).toHaveBeenCalledWith(
      'cursor',
      expect.objectContaining({ x: 20, y: 5 }),
    );
  });

  it('renders persisted annotations and remote cursors inside the world container', () => {
    useCRDT.getState().addAnnotation({
      start_x: 0,
      start_y: 0,
      end_x: 10,
      end_y: 10,
      color: '#00E5FF',
      author_id: 1,
    });
    useCRDT.setState({
      cursors: new Map([
        [999, { id: 999, color: '#FF00FF', x: 1, y: 2, name: 'Remote' }],
      ]),
    });

    render(<InteractiveCanvas />);
    // One Graphics for the annotation line, one for the remote cursor.
    expect(screen.getAllByTestId('pixi-graphics').length).toBeGreaterThanOrEqual(2);
  });
});
