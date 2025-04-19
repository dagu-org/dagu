import * as React from 'react';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '../contexts/UserPreference'; // Import the hook
import { mainListItems as MainListItems } from '../menu';
import { AppBarContext } from '../contexts/AppBarContext';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'; // Import panel icons

// Utility: Get contrast color (black or white) for a given background color (hex, rgb, or named)
function getContrastColor(input?: string): string {
  if (!input) return '#000'; // Default to black if undefined or empty

  let hex = input.trim();

  // If it's a named color or rgb(a), convert to hex using a dummy element
  if (!/^#([A-Fa-f0-9]{3}){1,2}$/.test(hex)) {
    if (typeof window !== 'undefined') {
      const temp = document.createElement('div');
      temp.style.color = hex;
      document.body.appendChild(temp);
      const computed = getComputedStyle(temp).color;
      document.body.removeChild(temp);

      // computed is in format "rgb(r, g, b)" or "rgba(r, g, b, a)"
      const rgbMatch = computed.match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)/);
      if (rgbMatch && rgbMatch[1] && rgbMatch[2] && rgbMatch[3]) {
        const r = parseInt(rgbMatch[1], 10);
        const g = parseInt(rgbMatch[2], 10);
        const b = parseInt(rgbMatch[3], 10);
        const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
        return luminance > 0.4 ? '#000' : '#fff';
      }
    }
    // Fallback if not in browser or can't parse
    return '#fff';
  }

  // Remove hash if present
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
const drawerWidthClosed = 'w-16'; // 64px
const drawerWidthOpen = 'w-60'; // 240px
// Removed margin constants as they are no longer needed

type LayoutProps = {
  title: string;
  navbarColor: string;
  version: string;
  children?: React.ReactElement | React.ReactElement[];
};

// Main Content component including Sidebar and AppBar logic
function Content({ title, navbarColor, children }: LayoutProps) {
  const [scrolled, setScrolled] = React.useState(false);
  // Local state for current visual status, default closed before hydration
  const [isSidebarOpen, setIsSidebarOpen] = React.useState(false);
  const { preferences, updatePreference } = useUserPreferences(); // Use the context
  const containerRef = React.useRef<HTMLDivElement>(null);

  // Use the config value for the sidebar color
  const sidebarColor = navbarColor || '#4D6744';

  // Effect to set initial state based on preference and screen size (desktop only)
  React.useEffect(() => {
    const isDesktop = window.innerWidth >= 768; // Tailwind's md breakpoint
    if (isDesktop) {
      // On desktop, set initial state from preference (defaulting to true if unset)
      setIsSidebarOpen(preferences.isSidebarOpenDesktop ?? true);
    } else {
      // On mobile, always start closed
      setIsSidebarOpen(false);
    }
    // Run only once on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Effect to update preference when sidebar state changes (desktop only)
  React.useEffect(() => {
    const isDesktop = window.innerWidth >= 768;
    if (isDesktop) {
      updatePreference('isSidebarOpenDesktop', isSidebarOpen);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isSidebarOpen]);

  // Effect to handle scroll shadow on AppBar
  React.useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 0);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  // Effect to show sidebar when mouse is near the left edge (desktop only)
  React.useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      // Only on desktop
      if (window.innerWidth < 768) return;
      // If mouse is within 24px of the left edge and sidebar is closed, open it
      if (e.clientX <= 24 && !isSidebarOpen) {
        setIsSidebarOpen(true);
      }
    };
    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, [isSidebarOpen]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-white">
      {/* Sidebar */}
      <div
        className={cn(
          // Base styles: border, background
          'h-full overflow-hidden bg-white border-r border-gray-200',
          // Always fixed for slide-in effect
          'fixed inset-y-0 left-0 z-40 transform transition-transform duration-300 ease-in-out',
          isSidebarOpen ? 'translate-x-0' : '-translate-x-full',
          isSidebarOpen ? drawerWidthOpen : 'w-0'
        )}
        onMouseLeave={() => {
          if (window.innerWidth >= 768) setIsSidebarOpen(false);
        }}
      >
        {/* Wrap nav and button in a flex column to push button to bottom */}
        <div className="flex flex-col justify-between h-full">
          <nav className="p-3 pt-4">
            <MainListItems isOpen={isSidebarOpen} />
          </nav>
          {/* Desktop Toggle Button (Bottom Left) */}
          {/* Toggle button removed as per user request */}
        </div>
      </div>

      {/* Main Content Area - Now relies on flex-1 to fill space */}
      <div className="flex flex-col flex-1 h-full max-w-full overflow-hidden bg-gray-100">
        {/* AppBar */}
        <header
          className={cn(
            'relative w-full px-6 transition-shadow duration-200',
            scrolled
              ? 'shadow-md border-b border-gray-300'
              : 'border-b border-transparent'
          )}
          style={{
            backgroundColor:
              navbarColor && navbarColor.trim() !== ''
                ? navbarColor
                : '#4D6744',
            color: getContrastColor(
              navbarColor && navbarColor.trim() !== '' ? navbarColor : '#4D6744'
            ),
          }}
        >
          <div className="flex items-center justify-between w-full h-16">
            {/* Left side content: Toggle Button + Title */}
            <div className="flex items-center space-x-4">
              <button
                className="p-2 rounded-md hover:bg-white/10 focus:outline-none focus:ring-0 focus-visible:outline-none"
                style={{
                  color: getContrastColor(
                    navbarColor && navbarColor.trim() !== ''
                      ? navbarColor
                      : '#4D6744'
                  ),
                }}
                aria-label="Open sidebar"
                onClick={() => setIsSidebarOpen(true)}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  className="w-6 h-6"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
                  />
                </svg>
              </button>
              <AppBarContext.Consumer>
                {(context) => (
                  <NavBarTitleText
                    visible={scrolled}
                    color={getContrastColor(
                      navbarColor && navbarColor.trim() !== ''
                        ? navbarColor
                        : '#4D6744'
                    )}
                  >
                    {context.title}
                  </NavBarTitleText>
                )}
              </AppBarContext.Consumer>
            </div>

            {/* Right side content */}
            <div className="flex items-center space-x-4">
              <NavBarTitleText
                color={getContrastColor(
                  navbarColor && navbarColor.trim() !== ''
                    ? navbarColor
                    : '#4D6744'
                )}
              >
                {title || 'Dagu'}
              </NavBarTitleText>
              <AppBarContext.Consumer>
                {(context) => {
                  if (
                    !context.remoteNodes ||
                    context.remoteNodes.length === 0
                  ) {
                    return null;
                  }
                  return (
                    <Select
                      value={context.selectedRemoteNode}
                      onValueChange={context.selectRemoteNode}
                    >
                      <SelectTrigger className="w-32 bg-white text-gray-900 border border-gray-300 focus:ring-2 focus:ring-brand-green">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {context.remoteNodes.map((node) => (
                          <SelectItem key={node} value={node}>
                            {node}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  );
                }}
              </AppBarContext.Consumer>
            </div>
          </div>
        </header>

        {/* Scrollable Content */}
        <main
          ref={containerRef}
          className="flex-1 overflow-y-auto overflow-x-hidden px-6 py-6"
        >
          {children}
        </main>
      </div>
    </div>
  );
}

// NavBarTitleText component
type NavBarTitleTextProps = {
  children: string;
  visible?: boolean;
  color?: string;
};

const NavBarTitleText = ({
  children,
  visible = true,
  color = 'white',
}: NavBarTitleTextProps) => {
  return (
    <h1
      className={cn(
        'text-2xlg font-extrabold transition-opacity duration-200 whitespace-nowrap',
        visible ? 'opacity-100' : 'opacity-0'
      )}
      style={{ color }}
    >
      {children}
    </h1>
  );
};

// Default export Layout component
export default function Layout({ children, ...props }: LayoutProps) {
  return <Content {...props}>{children}</Content>;
}
