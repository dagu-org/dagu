import { cn } from '@/lib/utils';
import { Menu, X } from 'lucide-react';
import * as React from 'react';
import { mainListItems as MainListItems } from '../menu';

// Constants
const sidebarWidthCollapsed = 'w-12'; // 48px for icon-only sidebar

type LayoutProps = {
  children?: React.ReactElement | React.ReactElement[];
};

/**
 * Render the application's main layout with a responsive sidebar and scrollable content area.
 *
 * The desktop sidebar expansion state is persisted to `localStorage` under `sidebarExpanded`.
 *
 * @param children - Content rendered in the main scrollable area of the layout
 * @returns The JSX element for the full layout (sidebar and main content)
 */
function Content({ children }: LayoutProps) {
  // Sidebar state with localStorage persistence
  const [isSidebarExpanded, setIsSidebarExpanded] = React.useState(() => {
    const saved = localStorage.getItem('sidebarExpanded');
    return saved ? saved === 'true' : false;
  });
  // Mobile sidebar state (hidden by default)
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = React.useState(false);

  // Save sidebar state to localStorage when it changes
  React.useEffect(() => {
    localStorage.setItem('sidebarExpanded', isSidebarExpanded.toString());
  }, [isSidebarExpanded]);

  // Toggle sidebar function
  const toggleSidebar = () => {
    setIsSidebarExpanded(!isSidebarExpanded);
  };

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background">
      {/* Desktop Sidebar - Hidden on mobile, visible in collapsed state on desktop */}
      <div
        className={cn(
          // Modern base styles with sidebar background
          'h-full overflow-hidden bg-sidebar text-sidebar-foreground',
          // Hidden on mobile, visible on desktop
          'hidden md:block',
          'z-40 transition-all duration-200 ease-in-out',
          isSidebarExpanded ? 'w-60' : sidebarWidthCollapsed
        )}
      >
        {/* Simplified flex column layout */}
        <div className="flex flex-col h-full">
          <nav className="flex-1">
            <MainListItems
              isOpen={isSidebarExpanded}
              onToggle={toggleSidebar}
              // Don't collapse sidebar on navigation to prevent jarring transition
            />
          </nav>
        </div>
      </div>

      {/* Mobile Sidebar - Overlay that appears when hamburger menu is clicked */}
      {isMobileSidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-50 md:hidden flex"
          onClick={() => setIsMobileSidebarOpen(false)}
        >
          <div
            className="h-full w-60 bg-sidebar text-sidebar-foreground overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-end p-4">
              <button
                onClick={() => setIsMobileSidebarOpen(false)}
                className="text-sidebar-foreground hover:text-sidebar-foreground/70 transition-colors"
              >
                <X className="h-6 w-6" />
              </button>
            </div>
            <div className="flex flex-col h-full">
              <nav className="flex-1">
                <MainListItems
                  isOpen={true}
                  onNavItemClick={() => setIsMobileSidebarOpen(false)}
                />
              </nav>
            </div>
          </div>
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex flex-col flex-1 h-full overflow-hidden bg-muted/30">
        {/* Mobile menu button - floating */}
        <button
          className="md:hidden fixed top-3 left-3 z-40 p-2 rounded-md bg-sidebar text-sidebar-foreground hover:bg-sidebar/80 transition-colors shadow-md"
          onClick={() => setIsMobileSidebarOpen(true)}
          aria-label="Open menu"
        >
          <Menu className="h-5 w-5" />
        </button>

        {/* Scrollable Content */}
        <main className="flex-1 overflow-y-auto overflow-x-hidden px-6 py-6">
          {children}
        </main>
      </div>
    </div>
  );
}

// Default export Layout component
export default function Layout({ children }: LayoutProps) {
  return <Content>{children}</Content>;
}