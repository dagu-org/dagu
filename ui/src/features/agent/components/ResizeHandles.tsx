import type { ReactElement } from 'react';

import { useResizableDraggable } from '../hooks/useResizableDraggable';

type Props = {
  resizeHandlers: ReturnType<typeof useResizableDraggable>['resizeHandlers'];
};

export function ResizeHandles({ resizeHandlers }: Props): ReactElement {
  return (
    <>
      <div
        className="absolute top-0 left-2 right-2 h-1.5 cursor-n-resize"
        {...resizeHandlers.top}
      />
      <div
        className="absolute bottom-0 left-2 right-2 h-1.5 cursor-s-resize"
        {...resizeHandlers.bottom}
      />
      <div
        className="absolute left-0 top-2 bottom-2 w-1.5 cursor-w-resize"
        {...resizeHandlers.left}
      />
      <div
        className="absolute right-0 top-2 bottom-2 w-1.5 cursor-e-resize"
        {...resizeHandlers.right}
      />
      <div
        className="absolute top-0 left-0 w-3 h-3 cursor-nw-resize"
        {...resizeHandlers.topLeft}
      />
      <div
        className="absolute top-0 right-0 w-3 h-3 cursor-ne-resize"
        {...resizeHandlers.topRight}
      />
      <div
        className="absolute bottom-0 left-0 w-3 h-3 cursor-sw-resize"
        {...resizeHandlers.bottomLeft}
      />
      <div
        className="absolute bottom-0 right-0 w-3 h-3 cursor-se-resize"
        {...resizeHandlers.bottomRight}
      />
    </>
  );
}
