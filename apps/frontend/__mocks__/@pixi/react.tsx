/**
 * Test double for @pixi/react: renders the declarative PixiJS components as
 * plain DOM passthroughs so component trees can be smoke-tested in jsdom
 * without a WebGL context. Activated per test file via vi.mock('@pixi/react').
 */
import React from 'react';

/* eslint-disable @typescript-eslint/no-explicit-any */

export const Stage = ({ children }: any) => (
  <div data-testid="pixi-stage">{children}</div>
);

export const Container = ({ children }: any) => (
  <div data-testid="pixi-container">{children}</div>
);

export const Graphics = (props: any) => (
  <div
    data-testid="pixi-graphics"
    onPointerDown={props.pointerdown}
  />
);

export const Text = ({ text }: any) => (
  <span data-testid="pixi-text">{text}</span>
);
