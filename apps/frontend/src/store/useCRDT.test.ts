import { describe, it, expect, vi, beforeEach } from 'vitest';
import type { AnnotationVector, CursorState } from '@git-viz/shared-types';
import { useCRDT, resolveWsBase } from './useCRDT';

vi.mock('y-websocket');

/* eslint-disable @typescript-eslint/no-explicit-any */

const vector: AnnotationVector = {
  start_x: 1,
  start_y: 2,
  end_x: 30,
  end_y: 40,
  color: '#00E5FF',
  author_id: 7,
};

describe('useCRDT', () => {
  beforeEach(() => {
    useCRDT.getState().connect('repo_map_1');
  });

  it('resolveWsBase points at the local backend /ws path in dev (no query string)', () => {
    expect(resolveWsBase()).toBe('ws://localhost:8080/ws');
    expect(resolveWsBase()).not.toContain('?');
  });

  it('connect passes WS_BASE and the room as separate provider arguments', () => {
    const provider = useCRDT.getState().provider as any;
    expect(provider.url).toBe('ws://localhost:8080/ws');
    expect(provider.roomname).toBe('repo_map_1');
    expect(provider.doc).toBe(useCRDT.getState().ydoc);
  });

  it('connect destroys the previous provider and resets room state', () => {
    const first = useCRDT.getState().provider as any;
    useCRDT.getState().connect('repo_map_2');
    expect(first.destroy).toHaveBeenCalled();
    expect((useCRDT.getState().provider as any).roomname).toBe('repo_map_2');
    expect(useCRDT.getState().annotations).toEqual([]);
  });

  it('addAnnotation pushes into the shared annotations Y.Array and the subscription mirrors it into state', () => {
    expect(useCRDT.getState().annotations).toEqual([]);

    useCRDT.getState().addAnnotation(vector);
    expect(useCRDT.getState().annotations).toEqual([vector]);

    // Mutations applied directly to the Y.Array (e.g. a remote update)
    // propagate through the same observer.
    const remote: AnnotationVector = { ...vector, author_id: 99 };
    useCRDT.getState().ydoc.getArray<AnnotationVector>('annotations').push([remote]);
    expect(useCRDT.getState().annotations).toEqual([vector, remote]);
  });

  it('updateCursor broadcasts the world-space pointer through awareness', () => {
    useCRDT.getState().updateCursor(123, 456);
    const provider = useCRDT.getState().provider as any;
    expect(provider.awareness.setLocalStateField).toHaveBeenCalledWith(
      'cursor',
      expect.objectContaining({ x: 123, y: 456, color: '#00E5FF' }),
    );
  });

  it('mirrors remote awareness cursors into state, excluding the local client', () => {
    const provider = useCRDT.getState().provider as any;
    const ydoc = useCRDT.getState().ydoc;
    const changeHandler = provider.awareness.on.mock.calls.find(
      (c: any[]) => c[0] === 'change',
    )?.[1] as () => void;
    expect(changeHandler).toBeTypeOf('function');

    const remoteCursor: CursorState = { id: 999, color: '#FF00FF', x: 5, y: 6, name: 'Remote' };
    const ownCursor: CursorState = { id: ydoc.clientID, color: '#00E5FF', x: 1, y: 1, name: 'Me' };
    provider.awareness.getStates.mockReturnValue(
      new Map<number, { cursor?: CursorState }>([
        [999, { cursor: remoteCursor }],
        [ydoc.clientID, { cursor: ownCursor }],
        [555, {}], // collaborator without a cursor yet
      ]),
    );

    changeHandler();

    const cursors = useCRDT.getState().cursors;
    expect(cursors.get(999)).toEqual(remoteCursor);
    expect(cursors.has(ydoc.clientID)).toBe(false);
    expect(cursors.has(555)).toBe(false);
  });
});
