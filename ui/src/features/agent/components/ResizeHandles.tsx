import type { ReactElement } from 'react';

import { useResizableDraggable } from '../hooks/useResizableDraggable';

type Props = {
  resizeHandlers: ReturnType<typeof useResizableDraggable>['resizeHandlers'];
};

export function ResizeHandles({ resizeHandlers }: Props): ReactElement {
  return (
    <>
      <div
        className="absolute top-0 left-3 right-3 h-3 cursor-n-resize z-10"
        {...resizeHandlers.top}
      />
      <div
        className="absolute bottom-0 left-3 right-3 h-3 cursor-s-resize z-10"
        {...resizeHandlers.bottom}
      />
      <div
        className="absolute left-0 top-3 bottom-3 w-3 cursor-w-resize z-10"
        {...resizeHandlers.left}
      />
      <div
        className="absolute right-0 top-3 bottom-3 w-3 cursor-e-resize z-10"
        {...resizeHandlers.right}
      />
      <div
        className="absolute top-0 left-0 w-4 h-4 cursor-nw-resize z-10"
        {...resizeHandlers.topLeft}
      />
      <div
        className="absolute top-0 right-0 w-4 h-4 cursor-ne-resize z-10"
        {...resizeHandlers.topRight}
      />
      <div
        className="absolute bottom-0 left-0 w-4 h-4 cursor-sw-resize z-10"
        {...resizeHandlers.bottomLeft}
      />
      <div
        className="absolute bottom-0 right-0 w-4 h-4 cursor-se-resize z-10"
        {...resizeHandlers.bottomRight}
      />
    </>
  );
}
