import * as React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { BarChart2, Search, List } from 'lucide-react';
import { cn } from '@/lib/utils'; // Assuming cn utility is available

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
        'flex items-center justify-center w-5 h-5',
        isActive ? 'text-white' : 'text-[#7EB36A]' // Match text color
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
>(({ isOpen = false, onNavItemClick }, ref) => (
  <div ref={ref} className="flex flex-col h-full">
    {/* Modern Sidebar Header */}
    <div className="flex items-center gap-2 px-4 py-4 mb-2">
      {/* Simplified SVG Logo */}
      <span className="flex items-center justify-center w-8 h-8">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 100 100"
          width="32"
          height="32"
        >
          {/* Main circle with dark green color */}
          <circle cx="50" cy="50" r="45" fill="#4a6741" />
          {/* Inner ring with slightly lighter color and a gap */}
          <path
            d="M50 20
                   A30 30 0 1 1 20 50
                   A30 30 0 1 1 80 50"
            stroke="white"
            strokeWidth="6"
            fill="none"
            strokeLinecap="round"
          />
        </svg>
      </span>
      <span
        className={cn(
          'font-bold tracking-wide select-none transition-opacity duration-200',
          isOpen ? 'text-2xl text-white opacity-100' : 'opacity-0 text-lg'
        )}
      >
        Dagu
      </span>
    </div>
    {/* Modern Navigation */}
    <nav className="flex-1 flex flex-col gap-2 py-2 px-3">
      <NavItem
        to="/dashboard"
        text="Dashboard"
        icon={<BarChart2 size={18} />}
        isOpen={isOpen}
        onClick={onNavItemClick}
      />
      <NavItem
        to="/dags"
        text="DAGs"
        icon={<List size={18} />}
        isOpen={isOpen}
        onClick={onNavItemClick}
      />
      <NavItem
        to="/search"
        text="Search"
        icon={<Search size={18} />}
        isOpen={isOpen}
        onClick={onNavItemClick}
      />
    </nav>
    {/* Optional: Sidebar Footer */}
    {/* Footer removed as per user request */}
  </div>
));
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

  return (
    <Link
      to={to}
      onClick={onClick}
      className={cn(
        'flex items-center px-3 py-2.5 text-sm font-medium rounded-lg transition-all duration-200 group whitespace-nowrap overflow-hidden relative',
        isActive
          ? 'text-white bg-white/10' // Active: subtle background
          : 'text-[#7EB36A] hover:text-white hover:bg-white/5' // Inactive: lighter green for better contrast on dark background
      )}
      aria-current={isActive ? 'page' : undefined}
    >
      {/* Active indicator - left border */}
      {isActive && (
        <span className="absolute left-0 top-0 bottom-0 w-0.5 bg-white rounded-full" />
      )}

      <Icon isActive={isActive}>{icon}</Icon>

      {/* Text with improved transition */}
      <span
        className={cn(
          'ml-3 transition-all duration-300 ease-in-out font-medium text-white',
          isOpen
            ? 'opacity-100 translate-x-0'
            : 'opacity-0 -translate-x-4 absolute' // Slide and fade
        )}
        aria-hidden={!isOpen}
      >
        {text}
      </span>
    </Link>
  );
}
