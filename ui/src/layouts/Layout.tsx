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
const sidebarWidthCollapsed = 'w-12'; // 48px for icon-only sidebar

type LayoutProps = {
  title: string;
  navbarColor: string;
  version: string;
  children?: React.ReactElement | React.ReactElement[];
};

// Main Content component including Sidebar and AppBar logic
function Content({ title, navbarColor, children }: LayoutProps) {
  const [scrolled, setScrolled] = React.useState(false);
  // Sidebar is always visible in collapsed state by default
  const [isSidebarExpanded, setIsSidebarExpanded] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);

  // Effect to handle scroll shadow on AppBar
  React.useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 0);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-white">
      {/* Sidebar - Always visible in collapsed state */}
      <div
        className={cn(
          // Modern base styles with dark background
          'h-full overflow-hidden bg-[#1E293B] text-white',
          // Shadow effect
          'shadow-lg',
          // Always visible, not fixed
          'z-40 transition-all duration-300 ease-in-out',
          isSidebarExpanded ? 'w-60' : sidebarWidthCollapsed
        )}
        onMouseEnter={() => setIsSidebarExpanded(true)}
        onMouseLeave={() => setIsSidebarExpanded(false)}
      >
        {/* Simplified flex column layout */}
        <div className="flex flex-col h-full">
          <nav className="flex-1">
            <MainListItems
              isOpen={isSidebarExpanded}
              onNavItemClick={() => setIsSidebarExpanded(false)}
            />
          </nav>

          {/* No bottom icon needed */}
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex flex-col flex-1 h-full overflow-hidden bg-gray-100">
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
          <div className="flex items-center justify-between w-full h-10">
            {/* Left side content: Title */}
            <div className="flex items-center space-x-2">
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
                      <SelectTrigger className="h-6 text-base bg-transparent text-white border-0 hover:bg-white/10 focus:ring-0 focus:outline-none focus:border-0 focus-visible:ring-0 focus-visible:outline-none focus-visible:border-0 focus-visible:shadow-none active:border-0 px-2 py-0 flex items-center min-h-0 gap-1 rounded-md transition-colors duration-200 [&_svg]:!text-white [&_svg]:!opacity-100 shadow-none cursor-pointer">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent className="text-base rounded-md overflow-hidden p-1 bg-gray-800 border-0 shadow-lg outline-none ring-0 focus:outline-none focus:ring-0 focus:border-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:border-0 text-white">
                        {context.remoteNodes.map((node) => (
                          <SelectItem
                            key={node}
                            value={node}
                            className="text-base py-1 px-2 min-h-0 h-8 text-white hover:bg-white/10 focus:bg-white/10 focus:outline-none focus:ring-0 focus:border-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:border-0 data-[highlighted]:bg-white/10 data-[highlighted]:text-white data-[selected]:text-white data-[selected]:bg-white/20 cursor-pointer"
                          >
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
  size?: 'sm' | 'base';
};

const NavBarTitleText = ({
  children,
  visible = true,
  color = 'white',
  size = 'base',
}: NavBarTitleTextProps) => {
  return (
    <h1
      className={cn(
        'font-medium transition-opacity duration-200 whitespace-nowrap',
        size === 'sm' ? 'text-sm' : 'text-base',
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
