import React, { useCallback, useEffect, useRef, useState } from 'react';

interface SplitLayoutProps {
  leftPanel: React.ReactNode;
  rightPanel: React.ReactNode | null;
  defaultLeftWidth?: number; // percentage (0-100)
  minLeftWidth?: number; // percentage
  maxLeftWidth?: number; // percentage
  storageKey?: string; // localStorage key for persistence
  emptyRightMessage?: string;
}

/**
 * SplitLayout component provides a master-detail view with
 * a list on the left and details on the right.
 * The divider is draggable and position is saved to localStorage.
 *
 * On mobile (<768px), only the left panel is shown and
 * detail navigation should happen via full-page routing.
 */
function SplitLayout({
  leftPanel,
  rightPanel,
  defaultLeftWidth = 40,
  minLeftWidth = 20,
  maxLeftWidth = 70,
  storageKey = 'splitLayoutWidth',
  emptyRightMessage = 'Select an item to view details',
}: SplitLayoutProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [leftWidth, setLeftWidth] = useState<number>(() => {
    const saved = localStorage.getItem(storageKey);
    if (saved) {
      const parsed = parseFloat(saved);
      if (!isNaN(parsed) && parsed >= minLeftWidth && parsed <= maxLeftWidth) {
        return parsed;
      }
    }
    return defaultLeftWidth;
  });
  const [isDragging, setIsDragging] = useState(false);

  // Save to localStorage when width changes
  useEffect(() => {
    localStorage.setItem(storageKey, leftWidth.toString());
  }, [leftWidth, storageKey]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!isDragging || !containerRef.current) return;

      const containerRect = containerRef.current.getBoundingClientRect();
      const newWidth =
        ((e.clientX - containerRect.left) / containerRect.width) * 100;

      // Clamp to min/max
      const clampedWidth = Math.min(
        maxLeftWidth,
        Math.max(minLeftWidth, newWidth)
      );
      setLeftWidth(clampedWidth);
    },
    [isDragging, minLeftWidth, maxLeftWidth]
  );

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
  }, []);

  // Add/remove global mouse listeners when dragging
  useEffect(() => {
    if (isDragging) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
    } else {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    }

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
    };
  }, [isDragging, handleMouseMove, handleMouseUp]);

  return (
    <div
      ref={containerRef}
      className="flex h-[calc(100vh-48px)] overflow-hidden"
    >
      {/* Left Panel - Table/List */}
      <div
        className="h-full overflow-y-auto overflow-x-hidden w-full md:w-auto flex-shrink-0"
        style={{ width: `${leftWidth}%` }}
      >
        {leftPanel}
      </div>

      {/* Draggable Divider - Hidden on mobile */}
      <div
        className="hidden md:flex items-center justify-center w-1 h-full bg-border hover:bg-primary/50 cursor-col-resize flex-shrink-0 transition-colors group"
        onMouseDown={handleMouseDown}
      >
        <div
          className={`w-0.5 h-8 rounded-full transition-colors ${
            isDragging ? 'bg-primary' : 'bg-muted-foreground/30 group-hover:bg-primary/70'
          }`}
        />
      </div>

      {/* Right Panel - Details (hidden on mobile) */}
      <div className="hidden md:flex flex-1 h-full overflow-hidden min-w-0">
        {rightPanel ? (
          <div className="flex flex-col w-full h-full overflow-hidden bg-background">
            {rightPanel}
          </div>
        ) : (
          <div className="flex items-center justify-center w-full h-full bg-muted/20">
            <p className="text-muted-foreground text-sm">{emptyRightMessage}</p>
          </div>
        )}
      </div>
    </div>
  );
}

export default SplitLayout;
