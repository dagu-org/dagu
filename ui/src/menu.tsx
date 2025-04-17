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
  <div ref={ref} className="flex flex-col space-y-1">
    {' '}
    {/* Vertical layout with spacing */}
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
        'flex items-center px-4 py-3 text-sm font-medium rounded-md transition-colors duration-150 group whitespace-nowrap overflow-hidden', // Added whitespace-nowrap and overflow-hidden
        isActive
          ? 'bg-gray-200 text-gray-900' // Active state: background and text color
          : 'text-gray-600 hover:bg-gray-100 hover:text-gray-900' // Default & Hover state: text and background color
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
