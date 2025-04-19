import * as React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import {
  faChartGantt,
  faMagnifyingGlass,
  faTableList,
} from '@fortawesome/free-solid-svg-icons';
import { IconProp } from '@fortawesome/fontawesome-svg-core';
import { cn } from '@/lib/utils'; // Assuming cn utility is available

// Reusable Icon component (minor style adjustments for Tailwind context)
function Icon({ icon }: { icon: IconProp }) {
  return (
    <span className="flex items-center justify-center w-5 h-5">
      {' '}
      {/* Tailwind for centering and size */}
      <FontAwesomeIcon icon={icon} className="text-inherit" />{' '}
      {/* Inherit color */}
    </span>
  );
}

// Define props for mainListItems to accept isOpen
type MainListItemsProps = {
  isOpen?: boolean;
};

// Main navigation items structure - now accepts isOpen prop
export const mainListItems = React.forwardRef<
  HTMLDivElement,
  MainListItemsProps
>(({ isOpen = false }, ref) => (
  <div ref={ref} className="flex flex-col h-full">
    {/* Sidebar Header */}
    <div className="flex items-center gap-3 px-4 py-6 border-b border-gray-200 bg-white/80 align-middle">
      {/* SVG Logo */}
      <span className="flex items-center justify-center w-12 h-12">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 100 100"
          width="40"
          height="40"
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
      <span className="text-4xl font-bold text-[#4D6744] tracking-wide select-none">
        Dagu
      </span>
    </div>
    {/* Navigation */}
    <nav className="flex-1 flex flex-col gap-1 py-4 px-2 bg-transparent">
      <NavItem
        to="/dashboard"
        text="Dashboard"
        icon={faChartGantt}
        isOpen={isOpen}
      />
      <NavItem to="/dags" text="DAGs" icon={faTableList} isOpen={isOpen} />
      <NavItem
        to="/search"
        text="Search"
        icon={faMagnifyingGlass}
        isOpen={isOpen}
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
  icon: IconProp;
  text: string;
  isOpen: boolean; // Add isOpen prop
};

function NavItem({ to, icon, text, isOpen }: NavItemProps) {
  const location = useLocation();
  const isActive = location.pathname.startsWith(to); // Simple active state check

  return (
    <Link
      to={to}
      className={cn(
        'flex items-center px-4 py-3 text-sm font-medium rounded-md transition-colors duration-150 group whitespace-nowrap overflow-hidden',
        isActive
          ? 'bg-white bg-opacity-20 text-white font-bold' // Active: semi-transparent white bg, bold white text
          : 'text-white/80 hover:bg-white hover:bg-opacity-10 hover:text-white' // Inactive: semi-transparent white, lighter on hover
      )}
      aria-current={isActive ? 'page' : undefined}
    >
      <Icon icon={icon} />
      {/* Conditionally render text based on isOpen state with transition */}
      <span
        className={cn(
          'ml-3 transition-opacity duration-200 ease-in-out',
          isOpen ? 'opacity-100' : 'opacity-0' // Fade in/out text
        )}
        aria-hidden={!isOpen} // Hide from screen readers when collapsed
      >
        {text}
      </span>
    </Link>
  );
}
