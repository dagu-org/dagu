import logoDark from '@/assets/images/logo_dark.png';
import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils'; // Assuming cn utility is available
import { BarChart2, GitBranch, List, Search, Server } from 'lucide-react';
import * as React from 'react';
import { Link, useLocation } from 'react-router-dom';

// Reusable Icon component using Lucide React
function Icon({
  children,
  isActive,
}: {
  children: React.ReactNode;
  isActive?: boolean;
}) {
  return (
    <span
      className={cn(
        'flex items-center justify-center w-5 h-5 transform-none text-primary-foreground',
        isActive ? 'text-primary-foreground' : 'text-primary-foreground'
      )}
    >
      {children}
    </span>
  );
}

// Define props for mainListItems to accept isOpen and onNavItemClick
type MainListItemsProps = {
  isOpen?: boolean;
  onNavItemClick?: () => void;
};

// Main navigation items structure - now accepts isOpen prop
export const mainListItems = React.forwardRef<
  HTMLDivElement,
  MainListItemsProps
>(({ isOpen = false, onNavItemClick }, ref) => {
  // Get version from config at the top level of the component
  const config = useConfig();

  return (
    <div ref={ref} className="flex flex-col h-full">
      {/* Fixed height header with absolute positioning for fixed logo */}
      <div className="h-12 relative mb-2">
        {/* Fixed position logo that doesn't move */}
        <div className="absolute left-3 top-1/2 transform -translate-y-1/2 w-6 h-6 flex items-center justify-center z-10">
          <img
            src={logoDark}
            alt="Dagu Logo"
            className="w-6 h-6 object-contain"
          />
        </div>

        {/* Text container with instant hide/show */}
        {isOpen && (
          <div className="absolute inset-0 flex items-center pl-12">
            <span className="font-bold tracking-wide select-none text-xl text-primary-foreground">
              Dagu
            </span>
          </div>
        )}
      </div>
      {/* Navigation */}
      <nav className="flex-1 flex flex-col gap-2 py-2 px-2">
        <NavItem
          to="/dashboard"
          text="Dashboard"
          icon={<BarChart2 size={18} />}
          isOpen={isOpen}
          onClick={onNavItemClick}
        />
        <NavItem
          to="/dag-runs"
          text="DAG Runs"
          icon={<List size={18} />}
          isOpen={isOpen}
          onClick={onNavItemClick}
        />
        <NavItem
          to="/dags"
          text="DAG Definitions"
          icon={<GitBranch size={18} />}
          isOpen={isOpen}
          onClick={onNavItemClick}
        />
        <NavItem
          to="/search"
          text="Search DAG Definitions"
          icon={<Search size={18} />}
          isOpen={isOpen}
          onClick={onNavItemClick}
        />
        <NavItem
          to="/workers"
          text="Workers"
          icon={<Server size={18} />}
          isOpen={isOpen}
          onClick={onNavItemClick}
        />
      </nav>
      {/* Version display - only shown when sidebar is expanded */}
      {isOpen && (
        <div className="px-3 py-2 text-xs text-primary-foreground/60">
          <div className="border-t border-primary-foreground/10 pt-2">
            Version: {config.version}
          </div>
        </div>
      )}
    </div>
  );
});
mainListItems.displayName = 'MainListItems'; // Add display name for DevTools

// Refactored NavItem component using Tailwind
type NavItemProps = {
  to: string;
  icon: React.ReactNode;
  text: string;
  isOpen: boolean;
  onClick?: () => void; // Add onClick prop
};

function NavItem({ to, icon, text, isOpen, onClick }: NavItemProps) {
  const location = useLocation();
  const isActive = location.pathname.startsWith(to);

  // Use different layouts for expanded and collapsed states
  if (isOpen) {
    return (
      <div className="relative h-9">
        <Link
          to={to}
          onClick={onClick}
          className={cn(
            'block h-9 flex items-center text-xs font-medium rounded-lg transition-all duration-50 ease-in-out pl-10 pr-3',
            isActive
              ? 'text-primary-foreground bg-primary-foreground/10' // Active: subtle background
              : 'text-primary-foreground hover:text-primary-foreground hover:bg-primary-foreground/5' // Inactive: lighter green for better contrast
          )}
          aria-current={isActive ? 'page' : undefined}
          title={text}
        >
          {/* Icon with fixed position */}
          <div className="flex items-center justify-center absolute left-3 top-1/2 transform -translate-y-1/2">
            <Icon isActive={isActive}>{icon}</Icon>
          </div>

          {/* Text with fade-in animation */}
          <span className="font-medium text-primary-foreground text-xs ml-3">
            {text}
          </span>
        </Link>
      </div>
    );
  } else {
    return (
      <div className="flex justify-center">
        <Link
          to={to}
          onClick={onClick}
          className={cn(
            'flex items-center justify-center w-8 h-8 text-xs font-medium rounded-lg transition-all duration-50 ease-in-out',
            isActive
              ? 'text-primary-foreground bg-primary-foreground/10' // Active: subtle background
              : 'text-primary-foreground hover:text-primary-foreground hover:bg-primary-foreground/5'
          )}
          aria-current={isActive ? 'page' : undefined}
          title={text}
        >
          <Icon isActive={isActive}>{icon}</Icon>
        </Link>
      </div>
    );
  }
}
