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
  }, []); // Empty dependency array means run once on mount

  const toggleSidebar = () => {
    const nextState = !isSidebarOpen;
    setIsSidebarOpen(nextState); // Update visual state

    // If on desktop, update the preference
    const isDesktop = window.innerWidth >= 768;
    if (isDesktop) {
      updatePreference('isSidebarOpenDesktop', nextState);
    }
  };

  const handleScroll = () => {
    const container = containerRef.current;
    if (container) {
      setScrolled(container.scrollTop > 54);
    }
  };

  return (
    // Root container using flexbox
    <div className="flex flex-row w-screen h-screen">
      {/* Overlay for mobile sidebar */}
      {isSidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/50 md:hidden"
          onClick={toggleSidebar}
          aria-hidden="true"
        />
      )}

      {/* Sidebar */}
      <div
        className={cn(
          // Base styles: border, background
          'h-full overflow-hidden bg-white border-r border-gray-200',
          // Mobile (<md): absolute overlay, slide transition
          'fixed inset-y-0 left-0 z-40 transform transition-transform duration-300 ease-in-out md:relative md:translate-x-0',
          isSidebarOpen ? 'translate-x-0' : '-translate-x-full',
          // Desktop (md+): width based on state (no width transition)
          'md:block',
          isSidebarOpen ? drawerWidthOpen : drawerWidthClosed
        )}
      >
        {/* Wrap nav and button in a flex column to push button to bottom */}
        <div className="flex flex-col justify-between h-full">
          <nav className="p-3 pt-4">
            <MainListItems isOpen={isSidebarOpen} />
          </nav>
          {/* Desktop Toggle Button (Bottom Left) */}
          {/* Changed justify-center to justify-start */}
          <div className="hidden md:flex justify-start p-4 border-t border-gray-200">
            <button
              onClick={toggleSidebar}
              className="p-2 text-gray-500 rounded-md hover:bg-gray-100 hover:text-gray-800 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-gray-400"
              aria-label={isSidebarOpen ? 'Collapse sidebar' : 'Expand sidebar'}
            >
              {/* Replaced Chevrons with Panel icons */}
              {isSidebarOpen ? (
                <PanelLeftClose size={20} />
              ) : (
                <PanelLeftOpen size={20} />
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Main Content Area - Now relies on flex-1 to fill space */}
      <div className="flex flex-col flex-1 h-full max-w-full overflow-hidden bg-gray-100">
        {/* AppBar */}
        <header
          className={cn(
            'relative w-full bg-gray-100 px-6 transition-shadow duration-200',
            scrolled
              ? 'shadow-md border-b border-gray-300'
              : 'border-b border-transparent'
          )}
        >
          <div className="flex items-center justify-between w-full h-16">
            {/* Left side content: Toggle Button + Title */}
            <div className="flex items-center space-x-4">
              {/* Mobile Toggle Button (Hamburger in Header) */}
              <button
                onClick={toggleSidebar}
                className="p-2 text-gray-600 rounded-md hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-gray-500 md:hidden" // Hide on medium screens and up
                aria-label="Toggle sidebar"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={1.5}
                  stroke="currentColor"
                  className="w-6 h-6"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
                  />
                </svg>
              </button>
              <AppBarContext.Consumer>
                {(context) => (
                  <NavBarTitleText visible={scrolled}>
                    {context.title}
                  </NavBarTitleText>
                )}
              </AppBarContext.Consumer>
            </div>

            {/* Right side content */}
            <div className="flex items-center space-x-4">
              <NavBarTitleText>{title || 'Dagu'}</NavBarTitleText>
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
                      onValueChange={(value: string) => {
                        context.selectRemoteNode(value);
                      }}
                    >
                      <SelectTrigger className="w-[150px] h-8 bg-white border border-gray-300 rounded text-black text-sm focus:ring-offset-0 focus:ring-0">
                        <SelectValue placeholder="Select Node" />
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
          className="flex-1 overflow-auto pb-4"
          onScroll={handleScroll}
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
};

const NavBarTitleText = ({
  children,
  visible = true,
}: NavBarTitleTextProps) => (
  <h1
    className={cn(
      'text-lg font-extrabold text-gray-700 transition-opacity duration-200 whitespace-nowrap',
      visible ? 'opacity-100' : 'opacity-0'
    )}
  >
    {children}
  </h1>
);

// Default export Layout component
export default function Layout({ children, ...props }: LayoutProps) {
  return <Content {...props}>{children}</Content>;
}
