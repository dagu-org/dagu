import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils';
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
  let r = 0,
    g = 0,
    b = 0;
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
  const hasCustomColor: boolean = Boolean(
    navbarColor && navbarColor.trim() !== ''
  );
  const contrastColor = hasCustomColor
    ? getContrastColor(navbarColor)
    : undefined;
  const sidebarStyle = hasCustomColor
    ? ({
        backgroundColor: navbarColor,
        color: contrastColor,
        '--sidebar-text': contrastColor,
      } as React.CSSProperties)
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
    <div className="flex h-screen w-full overflow-hidden bg-background">
      {/* Sidebar - Desktop */}
      <aside
        className={cn(
          'hidden md:block h-full transition-all duration-300 ease-in-out border-r border-border/40 z-20',
          isSidebarExpanded ? 'w-60' : 'w-[68px]',
          !hasCustomColor && 'bg-sidebar text-sidebar-foreground',
          hasCustomColor && 'custom-sidebar-color'
        )}
        style={sidebarStyle}
      >
        <div className="flex flex-col h-full">
          <nav className="flex-1 px-3 py-4">
            <MainListItems
              isOpen={isSidebarExpanded}
              onToggle={toggleSidebar}
              customColor={hasCustomColor}
            />
          </nav>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex flex-col flex-1 h-full overflow-hidden relative bg-background/50">
        {/* Mobile Header Bar */}
        <header
          className={cn(
            'md:hidden flex items-center justify-between h-14 px-4 flex-shrink-0 border-b border-border/40',
            !hasCustomColor && 'bg-sidebar text-sidebar-foreground',
            hasCustomColor && 'custom-sidebar-color'
          )}
          style={sidebarStyle}
        >
          <button
            className="p-2 rounded-md hover:bg-white/5 transition-colors"
            onClick={() => setIsMobileSidebarOpen(true)}
            aria-label="Open menu"
          >
            <Menu className="h-5 w-5" />
          </button>
          <span className="font-semibold text-sm tracking-tight uppercase opacity-80">
            {config.title || 'Dagu'}
          </span>
          <div className="w-8" />
        </header>

        {/* Scrollable Content */}
        <main className="flex-1 overflow-y-auto overflow-x-hidden p-4 md:p-6 lg:p-8">
          <div className="w-full h-full">{children}</div>
        </main>
      </div>

      {/* Mobile Sidebar - Overlay */}
      {isMobileSidebarOpen && (
        <div
          className="fixed inset-0 bg-background/80 z-50 md:hidden flex backdrop-blur-md"
          onClick={() => setIsMobileSidebarOpen(false)}
        >
          <div
            className={cn(
              'h-full w-64 overflow-hidden shadow-2xl border-r border-border/40',
              !hasCustomColor && 'bg-sidebar text-sidebar-foreground',
              hasCustomColor && 'custom-sidebar-color'
            )}
            style={sidebarStyle}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-between items-center p-4 border-b border-border/10">
              <span className="font-bold text-lg">
                {config.title || 'Dagu'}
              </span>
              <button
                onClick={() => setIsMobileSidebarOpen(false)}
                className="p-1 hover:bg-white/5 rounded-md transition-colors"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <div className="flex flex-col h-full pt-2">
              <nav className="flex-1 px-3">
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
    </div>
  );
}

// Default export Layout component
export default function Layout({ navbarColor, children }: LayoutProps) {
  return <Content navbarColor={navbarColor}>{children}</Content>;
}
