import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';
import { Menu, X } from 'lucide-react';
import * as React from 'react';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import { mainListItems as MainListItems } from '../menu';
import { ThemeToggle } from '@/components/ui/theme-toggle';
import { UserMenu } from '@/components/UserMenu';

/**
 * Choose a readable foreground color (black or white) that contrasts with the given background color.
 *
 * Accepts CSS color inputs such as hex (#rgb or #rrggbb), named colors, or rgb/rgba strings. When the input
 * cannot be parsed or when executed outside a browser environment, a conservative fallback is used.
 *
 * @param input - Background color value to evaluate (hex string, named color, or rgb/rgba). If omitted or empty, black is assumed.
 * @returns `'#000'` for a dark foreground (black) when the background is light, `'#fff'` for a light foreground (white) when the background is dark.
 */
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
const sidebarWidthCollapsed = 'w-12'; // 48px for icon-only sidebar

type LayoutProps = {
  title: string;
  navbarColor: string;
  version: string;
  children?: React.ReactElement | React.ReactElement[];
};

/**
 * Render the application's main layout including a responsive sidebar, app bar, and scrollable content area.
 *
 * The header uses `navbarColor` when provided and computes an appropriate contrast color for its text.
 * The desktop sidebar expansion state is persisted to `localStorage` under `sidebarExpanded`.
 *
 * @param title - Text displayed in the app bar as the primary title
 * @param navbarColor - Optional CSS color used as the app bar background; contrast text color is derived automatically
 * @param children - Content rendered in the main scrollable area of the layout
 * @returns The JSX element for the full layout (sidebar, app bar, and main content)
 */
function Content({ title, navbarColor, children }: LayoutProps) {
  const config = useConfig();
  const [scrolled, setScrolled] = React.useState(false);
  // Sidebar state with localStorage persistence
  const [isSidebarExpanded, setIsSidebarExpanded] = React.useState(() => {
    const saved = localStorage.getItem('sidebarExpanded');
    return saved ? saved === 'true' : false;
  });
  // Mobile sidebar state (hidden by default)
  const [isMobileSidebarOpen, setIsMobileSidebarOpen] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);

  // Effect to handle scroll shadow on AppBar
  React.useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 0);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

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
          // Modern base styles with dark background
          'h-full overflow-hidden bg-primary text-primary-foreground',
          // Shadow effect
          'shadow-lg',
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
            className="h-full w-60 bg-primary text-primary-foreground shadow-lg overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex justify-end p-4">
              <button
                onClick={() => setIsMobileSidebarOpen(false)}
                className="text-primary-foreground hover:text-primary-foreground/70 transition-colors"
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
        {/* AppBar */}
        <header
          className={cn(
            'relative w-full px-6 transition-shadow duration-200',
            scrolled
              ? 'shadow-md border-b border-border'
              : 'border-b border-transparent',
            // Use theme-aware classes when no custom color is set
            !navbarColor || navbarColor.trim() === ''
              ? 'bg-primary text-primary-foreground'
              : ''
          )}
          style={
            navbarColor && navbarColor.trim() !== ''
              ? {
                  backgroundColor: navbarColor,
                  color: getContrastColor(navbarColor),
                }
              : undefined
          }
        >
          <div className="flex items-center justify-between w-full h-10">
            {/* Left side content: Hamburger menu for mobile + Title */}
            <div className="flex items-center space-x-2">
              {/* Hamburger menu - only visible on mobile */}
              <button
                className="md:hidden text-current p-1 -ml-1 rounded-md hover:bg-accent/10 transition-colors"
                onClick={() => setIsMobileSidebarOpen(true)}
                aria-label="Open menu"
              >
                <Menu className="h-5 w-5" />
              </button>

              <NavBarTitleText
                color={
                  navbarColor && navbarColor.trim() !== ''
                    ? getContrastColor(navbarColor)
                    : undefined
                }
              >
                {title || ''}
              </NavBarTitleText>
              <AppBarContext.Consumer>
                {(context) => (
                  <NavBarTitleText
                    visible={scrolled}
                    color={
                      navbarColor && navbarColor.trim() !== ''
                        ? getContrastColor(navbarColor)
                        : undefined
                    }
                  >
                    {context.title}
                  </NavBarTitleText>
                )}
              </AppBarContext.Consumer>
            </div>

            {/* Right side content */}
            <div className="flex items-center space-x-2">
              <AppBarContext.Consumer>
                {(context) => {
                  // Hide remote node selector when builtin auth is enabled (auth is local-only)
                  if (
                    config.authMode === 'builtin' ||
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
                      <SelectTrigger className="h-7 text-sm !bg-transparent text-current border-0 hover:!bg-transparent dark:hover:!bg-transparent focus:!bg-transparent dark:focus:!bg-transparent focus:ring-0 focus:outline-none focus:border-0 focus-visible:ring-0 focus-visible:outline-none focus-visible:border-0 focus-visible:shadow-none active:border-0 px-2 py-0 flex items-center min-h-0 gap-1 rounded-md transition-colors duration-200 [&_svg]:!text-current [&_svg]:!opacity-100 shadow-none cursor-pointer">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className="text-sm rounded-md overflow-hidden p-1 bg-popover border-border shadow-lg outline-none ring-0 focus:outline-none focus:ring-0 focus:border-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:border-0 text-popover-foreground">
                        {context.remoteNodes.map((node) => (
                          <SelectItem
                            key={node}
                            value={node}
                            className="text-sm py-1 px-2 min-h-0 h-7 text-popover-foreground hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground focus:outline-none focus:ring-0 focus:border-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:border-0 data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground data-[selected]:bg-accent data-[selected]:text-accent-foreground cursor-pointer"
                          >
                            {node}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  );
                }}
              </AppBarContext.Consumer>
              <ThemeToggle />
              <UserMenu />
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
  size?: 'sm' | 'base';
};

const NavBarTitleText = ({
  children,
  visible = true,
  color,
  size = 'base',
}: NavBarTitleTextProps) => {
  return (
    <h1
      className={cn(
        'font-medium transition-opacity duration-200 whitespace-nowrap',
        size === 'sm' ? 'text-sm' : 'text-base',
        visible ? 'opacity-100' : 'opacity-0'
      )}
      style={color ? { color } : undefined}
    >
      {children}
    </h1>
  );
};

// Default export Layout component
export default function Layout({ children, ...props }: LayoutProps) {
  return <Content {...props}>{children}</Content>;
}