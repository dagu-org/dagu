import * as React from 'react';
import { cn } from '@/lib/utils'; // Assuming shadcn/ui setup includes this utility
import { mainListItems } from './menu'; // Assuming this renders compatible elements
import { AppBarContext } from './contexts/AppBarContext';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'; // Import shadcn Select

// Constants (can be adjusted or moved to Tailwind config if preferred)
const drawerWidthClosed = 'w-16'; // Equivalent to 64px

type LayoutProps = {
  title: string;
  navbarColor: string; // Keep prop, but might be used differently or removed if not needed with Tailwind
  version: string; // Keep prop, might be used elsewhere or in footer later
  children?: React.ReactElement | React.ReactElement[];
};

function Content({ title, navbarColor, children }: LayoutProps) {
  const [scrolled, setScrolled] = React.useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);
  // Use navbarColor for gradient, default if not provided
  const gradientColor = navbarColor || '#485fc7'; // Example default, adjust as needed

  const handleScroll = () => {
    const container = containerRef.current;
    if (container) {
      setScrolled(container.scrollTop > 54); // Keep scroll logic
    }
  };

  return (
    // No ThemeProvider needed for Tailwind
    <div className="flex flex-row w-screen h-screen">
      {/* Drawer */}
      <div
        className={cn(
          'relative h-full overflow-hidden',
          drawerWidthClosed // Fixed closed width as per original logic open={false}
        )}
      >
        <div
          className="h-full"
          style={{
            background: `linear-gradient(0deg, #fff 0%, ${gradientColor} 70%, ${gradientColor} 100%)`,
          }}
        >
          <nav className="pl-1.5 pt-2">{mainListItems}</nav>{' '}
          {/* Adjust padding as needed */}
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex flex-col flex-1 h-full max-w-full overflow-hidden bg-white">
        {/* AppBar */}
        <header
          className={cn(
            'relative w-full bg-gray-100 px-6 transition-shadow duration-200', // Use Tailwind bg color, padding
            scrolled
              ? 'shadow-md border-b border-gray-300'
              : 'border-b border-transparent' // Conditional border/shadow
          )}
        >
          <div className="flex items-center justify-between w-full h-16">
            {' '}
            {/* Use Tailwind flex for Toolbar */}
            {/* Left side title (conditionally visible) */}
            <AppBarContext.Consumer>
              {(context) => (
                <NavBarTitleText visible={scrolled}>
                  {context.title}
                </NavBarTitleText>
              )}
            </AppBarContext.Consumer>
            {/* Right side content */}
            <div className="flex items-center space-x-4">
              {' '}
              {/* Use Tailwind flex and spacing */}
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
          className="flex-1 overflow-auto bg-gray-100 pb-4" // Use Tailwind bg color, padding
          onScroll={handleScroll}
        >
          {children}
        </main>
      </div>
    </div>
  );
}

// Refactored NavBarTitleText using Tailwind
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
      'text-lg font-extrabold text-gray-700 transition-opacity duration-200', // Tailwind typography and transition
      visible ? 'opacity-100' : 'opacity-0' // Conditional opacity
    )}
  >
    {children}
  </h1>
);

// Export the Layout component
export default function Layout({ children, ...props }: LayoutProps) {
  return <Content {...props}>{children}</Content>;
}
