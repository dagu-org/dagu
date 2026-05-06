// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useCallback, useEffect, useRef } from 'react';

interface Bounds {
  right: number;
  bottom: number;
  width: number;
  height: number;
}

interface UseResizableDraggableOptions {
  defaultWidth?: number;
  defaultHeight?: number;
  defaultRight?: number;
  defaultBottom?: number;
  minWidth?: number;
  minHeight?: number;
  storageKey?: string;
}

const VIEWPORT_HORIZONTAL_PADDING = 32;
const VIEWPORT_VERTICAL_PADDING = 100;

type ResizeDirection =
  | 'top'
  | 'right'
  | 'bottom'
  | 'left'
  | 'topLeft'
  | 'topRight'
  | 'bottomLeft'
  | 'bottomRight';

function finiteNumber(value: unknown, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(value, max));
}

function clampBounds(
  bounds: Bounds,
  minWidth: number,
  minHeight: number
): Bounds {
  if (typeof window === 'undefined') {
    return bounds;
  }

  const maxWidth = Math.max(1, window.innerWidth - VIEWPORT_HORIZONTAL_PADDING);
  const maxHeight = Math.max(1, window.innerHeight - VIEWPORT_VERTICAL_PADDING);
  const width = clamp(bounds.width, Math.min(minWidth, maxWidth), maxWidth);
  const height = clamp(
    bounds.height,
    Math.min(minHeight, maxHeight),
    maxHeight
  );

  return {
    right: clamp(bounds.right, 0, Math.max(0, window.innerWidth - width)),
    bottom: clamp(bounds.bottom, 0, Math.max(0, window.innerHeight - height)),
    width,
    height,
  };
}

function areBoundsEqual(a: Bounds, b: Bounds): boolean {
  return (
    a.right === b.right &&
    a.bottom === b.bottom &&
    a.width === b.width &&
    a.height === b.height
  );
}

export function useResizableDraggable(
  options: UseResizableDraggableOptions = {}
) {
  const {
    defaultWidth = 440,
    defaultHeight = 540,
    defaultRight = 16,
    defaultBottom = 64,
    minWidth = 320,
    minHeight = 400,
    storageKey,
  } = options;

  const [bounds, setBounds] = useState<Bounds>(() => {
    const defaultBounds = {
      right: defaultRight,
      bottom: defaultBottom,
      width: defaultWidth,
      height: defaultHeight,
    };

    if (typeof window === 'undefined') {
      return defaultBounds;
    }
    try {
      const stored = storageKey ? localStorage.getItem(storageKey) : null;
      if (stored) {
        const parsed = JSON.parse(stored);
        return clampBounds(
          {
            right: finiteNumber(parsed.right, defaultRight),
            bottom: finiteNumber(parsed.bottom, defaultBottom),
            width: finiteNumber(parsed.width, defaultWidth),
            height: finiteNumber(parsed.height, defaultHeight),
          },
          minWidth,
          minHeight
        );
      }
    } catch {
      // Ignore parse errors
    }
    return clampBounds(defaultBounds, minWidth, minHeight);
  });

  const boundsRef = useRef(bounds);
  boundsRef.current = bounds;

  const dragStateRef = useRef<{
    isDragging: boolean;
    isResizing: boolean;
    resizeDirection: ResizeDirection | null;
    startX: number;
    startY: number;
    startBounds: Bounds;
  }>({
    isDragging: false,
    isResizing: false,
    resizeDirection: null,
    startX: 0,
    startY: 0,
    startBounds: bounds,
  });

  const saveBounds = useCallback(
    (b: Bounds) => {
      if (!storageKey) return;
      try {
        localStorage.setItem(storageKey, JSON.stringify(b));
      } catch {
        // Ignore storage errors
      }
    },
    [storageKey]
  );

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      const state = dragStateRef.current;
      if (!state.isDragging && !state.isResizing) return;

      const deltaX = e.clientX - state.startX;
      const deltaY = e.clientY - state.startY;
      const maxWidth = Math.max(
        1,
        window.innerWidth - VIEWPORT_HORIZONTAL_PADDING
      );
      const maxHeight = Math.max(
        1,
        window.innerHeight - VIEWPORT_VERTICAL_PADDING
      );

      if (state.isDragging) {
        // Dragging: move position (invert delta because we use right/bottom)
        setBounds((prev) =>
          clampBounds(
            {
              ...state.startBounds,
              width: prev.width,
              height: prev.height,
              right: Math.max(
                0,
                Math.min(
                  state.startBounds.right - deltaX,
                  window.innerWidth - prev.width
                )
              ),
              bottom: Math.max(
                0,
                Math.min(
                  state.startBounds.bottom - deltaY,
                  window.innerHeight - prev.height
                )
              ),
            },
            minWidth,
            minHeight
          )
        );
      } else if (state.isResizing && state.resizeDirection) {
        // Resizing: adjust size and possibly position
        const dir = state.resizeDirection;
        let newWidth = state.startBounds.width;
        let newHeight = state.startBounds.height;
        let newRight = state.startBounds.right;
        let newBottom = state.startBounds.bottom;

        // Handle horizontal resize
        if (dir.includes('Left') || dir === 'left') {
          // Left edge: increase width, keep right anchor
          newWidth = Math.max(
            minWidth,
            Math.min(state.startBounds.width - deltaX, maxWidth)
          );
        }
        if (dir.includes('Right') || dir === 'right') {
          // Right edge: increase width by moving right anchor
          newWidth = Math.max(
            minWidth,
            Math.min(state.startBounds.width + deltaX, maxWidth)
          );
          newRight = Math.max(0, state.startBounds.right - deltaX);
        }

        // Handle vertical resize
        if (dir.includes('top') || dir === 'top') {
          // Top edge: increase height, keep bottom anchor
          newHeight = Math.max(
            minHeight,
            Math.min(state.startBounds.height - deltaY, maxHeight)
          );
        }
        if (dir.includes('bottom') || dir === 'bottom') {
          // Bottom edge: increase height by moving bottom anchor
          newHeight = Math.max(
            minHeight,
            Math.min(state.startBounds.height + deltaY, maxHeight)
          );
          newBottom = Math.max(0, state.startBounds.bottom - deltaY);
        }

        setBounds(
          clampBounds(
            {
              right: newRight,
              bottom: newBottom,
              width: newWidth,
              height: newHeight,
            },
            minWidth,
            minHeight
          )
        );
      }
    },
    [minWidth, minHeight]
  );

  const handleMouseUp = useCallback(() => {
    const wasActive =
      dragStateRef.current.isDragging || dragStateRef.current.isResizing;
    dragStateRef.current.isDragging = false;
    dragStateRef.current.isResizing = false;
    dragStateRef.current.resizeDirection = null;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    if (wasActive) {
      saveBounds(boundsRef.current);
    }
  }, [saveBounds]);

  useEffect(() => {
    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [handleMouseMove, handleMouseUp]);

  useEffect(() => {
    const handleWindowResize = () => {
      setBounds((prev) => {
        const next = clampBounds(prev, minWidth, minHeight);
        return areBoundsEqual(prev, next) ? prev : next;
      });
    };

    window.addEventListener('resize', handleWindowResize);
    window.addEventListener('orientationchange', handleWindowResize);
    return () => {
      window.removeEventListener('resize', handleWindowResize);
      window.removeEventListener('orientationchange', handleWindowResize);
    };
  }, [minWidth, minHeight]);

  const startDrag = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragStateRef.current = {
        isDragging: true,
        isResizing: false,
        resizeDirection: null,
        startX: e.clientX,
        startY: e.clientY,
        startBounds: { ...bounds },
      };
      document.body.style.cursor = 'move';
      document.body.style.userSelect = 'none';
    },
    [bounds]
  );

  const startResize = useCallback(
    (direction: ResizeDirection) => (e: React.MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      dragStateRef.current = {
        isDragging: false,
        isResizing: true,
        resizeDirection: direction,
        startX: e.clientX,
        startY: e.clientY,
        startBounds: { ...bounds },
      };
      document.body.style.userSelect = 'none';
    },
    [bounds]
  );

  return {
    bounds,
    dragHandlers: { onMouseDown: startDrag },
    resizeHandlers: {
      top: { onMouseDown: startResize('top') },
      right: { onMouseDown: startResize('right') },
      bottom: { onMouseDown: startResize('bottom') },
      left: { onMouseDown: startResize('left') },
      topLeft: { onMouseDown: startResize('topLeft') },
      topRight: { onMouseDown: startResize('topRight') },
      bottomLeft: { onMouseDown: startResize('bottomLeft') },
      bottomRight: { onMouseDown: startResize('bottomRight') },
    },
  };
}
