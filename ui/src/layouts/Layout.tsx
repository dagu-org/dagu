import { cn } from '@/lib/utils';
import { useConfig } from '@/contexts/ConfigContext';
import { Menu, X } from 'lucide-react';
import * as React from 'react';
import { mainListItems as MainListItems } from '../menu';

/**
 * Choose a readable foreground color (black or white) that contrasts with the given background color.
 */
function getContrastColor(input?: string): string {
  if (!input) return '#000';

  let hex = input.trim();

  if (!/^#([A-Fa-f0-9]{3}){1,2}$/.test(hex)) {
    if (typeof window !== 'undefined') {
      const temp = document.createElement('div');
      temp.style.color = hex;
      document.body.appendChild(temp);
      const computed = getComputedStyle(temp).color;
      document.body.removeChild(temp);

      const rgbMatch = computed.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)/);
      if (rgbMatch && rgbMatch[1] && rgbMatch[2] && rgbMatch[3]) {
        const r = parseInt(rgbMatch[1], 10);
        const g = parseInt(rgbMatch[2], 10);
        const b = parseInt(rgbMatch[3], 10);
        const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
        return luminance > 0.4 ? '#000' : '#fff';
      }
    }
    return '#fff';
  }

  hex = hex.replace('#', '');
  let r = 0, g = 0, b = 0;
  if (hex.length === 3) {
    if (hex[0] && hex[1] && hex[2]) {
      r = parseInt(hex[0] + hex[0], 16);
      g = parseInt(hex[1] + hex[1], 16);
      b = parseInt(hex[2] + hex[2], 16);
    } else {
      return '#000';
    }
  } else if (hex.length === 6) {
    r = parseInt(hex.substring(0, 2), 16);
    g = parseInt(hex.substring(2, 4), 16);
    b = parseInt(hex.substring(4, 6), 16);
  } else {
    return '#000';
  }
  const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
  return luminance > 0.4 ? '#000' : '#fff';
}

// Constants
const sidebarWidthCollapsed = 'w-12'; // 48px for icon-only sidebar

type LayoutProps = {
  navbarColor?: string;
  children?: React.ReactElement | React.ReactElement[];
};

/**
 * Render the application's main layout with a responsive sidebar and scrollable content area.
 *
 * The desktop sidebar expansion state is persisted to `localStorage` under `sidebarExpanded`.
 * The sidebar uses `navbarColor` when provided and computes an appropriate contrast color for its text.
 *
 * @param navbarColor - Optional CSS color used as the sidebar background
 * @param children - Content rendered in the main scrollable area of the layout
 * @returns The JSX element for the full layout (sidebar and main content)
 */
function Content({ navbarColor, children }: LayoutProps) {
  const config = useConfig();
  const hasCustomColor: boolean = Boolean(navbarColor && navbarColor.trim() !== '');
  const contrastColor = hasCustomColor ? getContrastColor(navbarColor) : undefined;
  const sidebarStyle = hasCustomColor
    ? {
        backgroundColor: navbarColor,
        color: contrastColor,
        '--sidebar-text': contrastColor,
      } as React.CSSProperties
    : undefined;
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
          'h-full overflow-hidden',
          !hasCustomColor && 'bg-sidebar text-sidebar-foreground',
          hasCustomColor && 'custom-sidebar-color',
          // Hidden on mobile, visible on desktop
          'hidden md:block',
          'z-40 transition-all duration-200 ease-in-out',
          isSidebarExpanded ? 'w-60' : sidebarWidthCollapsed
        )}
        style={sidebarStyle}
      >
        {/* Simplified flex column layout */}
        <div className="flex flex-col h-full">
          <nav className="flex-1">
            <MainListItems
              isOpen={isSidebarExpanded}
              onToggle={toggleSidebar}
              customColor={hasCustomColor}
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
            className={cn(
              'h-full w-60 overflow-hidden',
              !hasCustomColor && 'bg-sidebar text-sidebar-foreground',
              hasCustomColor && 'custom-sidebar-color'
            )}
            style={sidebarStyle}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-end p-4">
              <button
                onClick={() => setIsMobileSidebarOpen(false)}
                className="text-current hover:opacity-70 transition-colors"
              >
                <X className="h-6 w-6" />
              </button>
            </div>
            <div className="flex flex-col h-full">
              <nav className="flex-1">
                <MainListItems
                  isOpen={true}
                  onNavItemClick={() => setIsMobileSidebarOpen(false)}
                  customColor={hasCustomColor}
                />
              </nav>
            </div>
          </div>
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex flex-col flex-1 h-full overflow-hidden bg-muted/30">
        {/* Mobile Header Bar */}
        <div
          className={cn(
            'md:hidden flex items-center justify-between h-12 px-3 flex-shrink-0',
            !hasCustomColor && 'bg-sidebar text-sidebar-foreground',
            hasCustomColor && 'custom-sidebar-color'
          )}
          style={sidebarStyle}
        >
          <button
            className="p-1.5 rounded-md hover:bg-white/10 transition-colors"
            onClick={() => setIsMobileSidebarOpen(true)}
            aria-label="Open menu"
          >
            <Menu className="h-5 w-5" />
          </button>
          <span className="font-bold text-lg">{config.title || 'Dagu'}</span>
          <div className="w-8" /> {/* Spacer for balance */}
        </div>

        {/* Scrollable Content */}
        <main className="flex-1 overflow-y-auto overflow-x-hidden px-4 md:px-6 py-4 md:py-6">
          {children}
        </main>
      </div>
    </div>
  );
}

// Default export Layout component
export default function Layout({ navbarColor, children }: LayoutProps) {
  return <Content navbarColor={navbarColor}>{children}</Content>;
}