import { describe, it, expect, beforeEach } from 'vitest';
import { useStore } from '../store/useStore';
import {
  captureViewState,
  serializeViewState,
  parseViewState,
  type CanvasViewState,
} from './viewState';

/** A fully-specified, valid view-state snapshot for round-trip tests. */
const sample: CanvasViewState = {
  viewport: { x: 12, y: -8, scale: 2.5 },
  searchQuery: 'alpha',
  tagsOnly: true,
  hiddenLanes: [1, 3],
  hiddenAuthors: ['Bob'],
  recompactLayout: true,
};

describe('viewState', () => {
  beforeEach(() => {
    useStore.setState({
      nodes: [],
      visibleNodes: [],
      searchQuery: '',
      serverHits: null,
      tagsOnly: false,
      hiddenLanes: [],
      hiddenAuthors: [],
      recompactLayout: false,
      viewportTransform: { x: 0, y: 0, scale: 1 },
      selectedNode: null,
      drawingState: false,
      token: null,
      dependencyLinks: [],
    });
  });

  describe('captureViewState', () => {
    it('reflects the current store viewport + active filters', () => {
      useStore.setState({
        viewportTransform: { x: 5, y: 6, scale: 1.5 },
        searchQuery: 'fix',
        tagsOnly: true,
        hiddenLanes: [2],
        hiddenAuthors: ['Alice'],
        recompactLayout: true,
      });

      expect(captureViewState()).toEqual({
        viewport: { x: 5, y: 6, scale: 1.5 },
        searchQuery: 'fix',
        tagsOnly: true,
        hiddenLanes: [2],
        hiddenAuthors: ['Alice'],
        recompactLayout: true,
      });
    });

    it('copies arrays so the snapshot does not alias live store state', () => {
      useStore.setState({ hiddenLanes: [1], hiddenAuthors: ['Bob'] });
      const snap = captureViewState();
      expect(snap.hiddenLanes).not.toBe(useStore.getState().hiddenLanes);
      expect(snap.hiddenAuthors).not.toBe(useStore.getState().hiddenAuthors);
    });
  });

  describe('serialize/parse round-trip', () => {
    it('parse(serialize(s)) deep-equals the original', () => {
      expect(parseViewState(serializeViewState(sample))).toEqual(sample);
    });
  });

  describe('parseViewState', () => {
    it('returns null on malformed JSON', () => {
      expect(parseViewState('{ not json')).toBeNull();
      expect(parseViewState('')).toBeNull();
    });

    it('returns null on a non-object payload', () => {
      expect(parseViewState('42')).toBeNull();
      expect(parseViewState('null')).toBeNull();
    });

    it('returns null when a required field is missing', () => {
      const { searchQuery, ...rest } = sample;
      void searchQuery;
      expect(parseViewState(JSON.stringify(rest))).toBeNull();
    });

    it('returns null when the viewport is mistyped', () => {
      const bad = { ...sample, viewport: { x: 'nope', y: 0, scale: 1 } };
      expect(parseViewState(JSON.stringify(bad))).toBeNull();
    });

    it('returns null when a boolean field is mistyped', () => {
      const bad = { ...sample, tagsOnly: 'yes' };
      expect(parseViewState(JSON.stringify(bad))).toBeNull();
    });

    it('returns null when an array field is not an array', () => {
      const bad = { ...sample, hiddenLanes: 3 };
      expect(parseViewState(JSON.stringify(bad))).toBeNull();
    });

    it('coerces array contents, dropping mistyped entries', () => {
      const messy = {
        ...sample,
        hiddenLanes: [1, 'x', 2, null],
        hiddenAuthors: ['Bob', 5, 'Carol'],
      };
      const parsed = parseViewState(JSON.stringify(messy));
      expect(parsed?.hiddenLanes).toEqual([1, 2]);
      expect(parsed?.hiddenAuthors).toEqual(['Bob', 'Carol']);
    });
  });
});
