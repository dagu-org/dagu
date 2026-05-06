// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { useResizableDraggable } from '../useResizableDraggable';

const originalInnerWidth = window.innerWidth;
const originalInnerHeight = window.innerHeight;

function setViewport(width: number, height: number): void {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    writable: true,
    value: width,
  });
  Object.defineProperty(window, 'innerHeight', {
    configurable: true,
    writable: true,
    value: height,
  });
}

afterEach(() => {
  localStorage.clear();
  setViewport(originalInnerWidth, originalInnerHeight);
});

describe('useResizableDraggable', () => {
  it('keeps stored modal bounds inside the current viewport', () => {
    setViewport(700, 520);
    localStorage.setItem(
      'agent-chat-modal-bounds',
      JSON.stringify({
        right: 620,
        bottom: 240,
        width: 560,
        height: 540,
      })
    );

    const { result } = renderHook(() =>
      useResizableDraggable({
        storageKey: 'agent-chat-modal-bounds',
        defaultWidth: 560,
      })
    );

    expect(result.current.bounds).toMatchObject({
      right: 140,
      bottom: 100,
      width: 560,
      height: 420,
    });
  });

  it('re-clamps open modal bounds when browser zoom changes the viewport', () => {
    setViewport(1100, 760);
    localStorage.setItem(
      'agent-chat-modal-bounds',
      JSON.stringify({
        right: 500,
        bottom: 260,
        width: 560,
        height: 540,
      })
    );

    const { result } = renderHook(() =>
      useResizableDraggable({
        storageKey: 'agent-chat-modal-bounds',
        defaultWidth: 560,
      })
    );

    setViewport(700, 520);
    act(() => {
      window.dispatchEvent(new Event('resize'));
    });

    expect(result.current.bounds).toMatchObject({
      right: 140,
      bottom: 100,
      width: 560,
      height: 420,
    });
  });
});
