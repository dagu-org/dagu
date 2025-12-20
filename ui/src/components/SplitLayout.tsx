import React from 'react';

interface SplitLayoutProps {
  leftPanel: React.ReactNode;
  rightPanel: React.ReactNode | null;
  leftWidth?: string;
  emptyRightMessage?: string;
}

/**
 * SplitLayout component provides a master-detail view with
 * a list on the left and details on the right.
 *
 * On mobile (<768px), only the left panel is shown and
 * detail navigation should happen via full-page routing.
 */
function SplitLayout({
  leftPanel,
  rightPanel,
  leftWidth = '40%',
  emptyRightMessage = 'Select an item to view details',
}: SplitLayoutProps) {
  return (
    <div className="flex h-[calc(100vh-48px)] overflow-hidden">
      {/* Left Panel - Table/List - Fixed width */}
      <div
        className="h-full overflow-y-auto overflow-x-hidden border-r border-border w-full md:w-auto"
        style={{ minWidth: leftWidth, maxWidth: leftWidth }}
      >
        {leftPanel}
      </div>

      {/* Right Panel - Details (hidden on mobile) - Takes remaining space */}
      <div className="hidden md:flex md:flex-1 h-full overflow-hidden">
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
