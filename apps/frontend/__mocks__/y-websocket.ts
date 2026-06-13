/**
 * Test double for y-websocket: records the constructor arguments (base URL,
 * room, doc) and exposes a spy-based awareness API so CRDT flows can be
 * exercised without a real socket. Activated via vi.mock('y-websocket').
 */
import { vi } from 'vitest';

/* eslint-disable @typescript-eslint/no-explicit-any */

export class WebsocketProvider {
  url: string;
  roomname: string;
  doc: any;

  awareness = {
    on: vi.fn(),
    setLocalStateField: vi.fn(),
    getStates: vi.fn(() => new Map()),
  };

  disconnect = vi.fn();
  destroy = vi.fn();

  constructor(url: string, roomname: string, doc: any) {
    this.url = url;
    this.roomname = roomname;
    this.doc = doc;
  }
}
