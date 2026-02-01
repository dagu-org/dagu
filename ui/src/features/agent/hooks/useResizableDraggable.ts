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

type ResizeDirection =
  | 'top'
  | 'right'
  | 'bottom'
  | 'left'
  | 'topLeft'
  | 'topRight'
  | 'bottomLeft'
  | 'bottomRight';

export function useResizableDraggable(options: UseResizableDraggableOptions = {}) {
  const {
    defaultWidth = 440,
    defaultHeight = 540,
    defaultRight = 16,
    defaultBottom = 64,
    minWidth = 320,
    minHeight = 400,
    storageKey = 'agent-chat-modal-bounds',
  } = options;

  const [bounds, setBounds] = useState<Bounds>(() => {
    if (typeof window === 'undefined') {
      return { right: defaultRight, bottom: defaultBottom, width: defaultWidth, height: defaultHeight };
    }
    try {
      const stored = localStorage.getItem(storageKey);
      if (stored) {
        const parsed = JSON.parse(stored);
        // Validate bounds are within viewport
        const maxWidth = window.innerWidth - 32;
        const maxHeight = window.innerHeight - 100;
        return {
          right: Math.max(0, Math.min(parsed.right ?? defaultRight, window.innerWidth - minWidth)),
          bottom: Math.max(0, Math.min(parsed.bottom ?? defaultBottom, window.innerHeight - minHeight)),
          width: Math.max(minWidth, Math.min(parsed.width ?? defaultWidth, maxWidth)),
          height: Math.max(minHeight, Math.min(parsed.height ?? defaultHeight, maxHeight)),
        };
      }
    } catch {
      // Ignore parse errors
    }
    return { right: defaultRight, bottom: defaultBottom, width: defaultWidth, height: defaultHeight };
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

  const saveBounds = useCallback((b: Bounds) => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(b));
    } catch {
      // Ignore storage errors
    }
  }, [storageKey]);

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      const state = dragStateRef.current;
      if (!state.isDragging && !state.isResizing) return;

      const deltaX = e.clientX - state.startX;
      const deltaY = e.clientY - state.startY;
      const maxWidth = window.innerWidth - 32;
      const maxHeight = window.innerHeight - 100;

      if (state.isDragging) {
        // Dragging: move position (invert delta because we use right/bottom)
        setBounds((prev) => ({
          ...prev,
          right: Math.max(0, Math.min(state.startBounds.right - deltaX, window.innerWidth - prev.width)),
          bottom: Math.max(0, Math.min(state.startBounds.bottom - deltaY, window.innerHeight - prev.height)),
        }));
      } else if (state.isResizing && state.resizeDirection) {
        // Resizing: adjust size and possibly position
        const dir = state.resizeDirection;
        let newWidth = state.startBounds.width;
        let newHeight = state.startBounds.height;
        let newRight = state.startBounds.right;
        let newBottom = state.startBounds.bottom;

        // Handle horizontal resize
        if (dir.includes('left') || dir === 'left') {
          // Left edge: increase width, keep right anchor
          newWidth = Math.max(minWidth, Math.min(state.startBounds.width - deltaX, maxWidth));
        }
        if (dir.includes('right') || dir === 'right') {
          // Right edge: increase width by moving right anchor
          newWidth = Math.max(minWidth, Math.min(state.startBounds.width + deltaX, maxWidth));
          newRight = Math.max(0, state.startBounds.right - deltaX);
        }

        // Handle vertical resize
        if (dir.includes('top') || dir === 'top') {
          // Top edge: increase height, keep bottom anchor
          newHeight = Math.max(minHeight, Math.min(state.startBounds.height - deltaY, maxHeight));
        }
        if (dir.includes('bottom') || dir === 'bottom') {
          // Bottom edge: increase height by moving bottom anchor
          newHeight = Math.max(minHeight, Math.min(state.startBounds.height + deltaY, maxHeight));
          newBottom = Math.max(0, state.startBounds.bottom - deltaY);
        }

        setBounds({ right: newRight, bottom: newBottom, width: newWidth, height: newHeight });
      }
    },
    [minWidth, minHeight]
  );

  const handleMouseUp = useCallback(() => {
    const wasActive = dragStateRef.current.isDragging || dragStateRef.current.isResizing;
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

  const startDrag = useCallback((e: React.MouseEvent) => {
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
  }, [bounds]);

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
