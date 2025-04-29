import { cn } from '@/lib/utils'; // Assuming cn utility is available
import { BarChart2, List, Search } from 'lucide-react';
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
        'flex items-center justify-center w-5 h-5 transform-none',
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
>(({ isOpen = false }, ref) => (
  <div ref={ref} className="flex flex-col h-full">
    {/* Fixed height header with absolute positioning for fixed logo */}
    <div className="h-12 relative mb-2">
      {/* Fixed position logo that doesn't move */}
      <div className="absolute left-3 top-1/2 transform -translate-y-1/2 w-6 h-6 flex items-center justify-center z-10">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 100 100"
          width="24"
          height="24"
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
      </div>

      {/* Text container with instant hide/show */}
      {isOpen && (
        <div className="absolute inset-0 flex items-center pl-12">
          <span className="font-bold tracking-wide select-none text-xl text-white">
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
      />
      <NavItem
        to="/dags"
        text="DAGs"
        icon={<List size={18} />}
        isOpen={isOpen}
      />
      <NavItem
        to="/search"
        text="Search"
        icon={<Search size={18} />}
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
            'block h-9 flex items-center text-xs font-medium rounded-lg transition-all duration-300 ease-in-out pl-10 pr-3',
            isActive
              ? 'text-white bg-white/10' // Active: subtle background
              : 'text-[#7EB36A] hover:text-white hover:bg-white/5' // Inactive: lighter green for better contrast
          )}
          aria-current={isActive ? 'page' : undefined}
          title={text}
        >
          {/* Icon with fixed position */}
          <div className="flex items-center justify-center absolute left-3 top-1/2 transform -translate-y-1/2">
            <Icon isActive={isActive}>{icon}</Icon>
          </div>

          {/* Text with fade-in animation */}
          <span className="font-medium text-white text-xs ml-3">{text}</span>
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
            'flex items-center justify-center w-8 h-8 text-xs font-medium rounded-lg transition-all duration-300 ease-in-out',
            isActive
              ? 'text-white bg-white/10' // Active: subtle background
              : 'text-[#7EB36A] hover:text-white hover:bg-white/5' // Inactive: lighter green for better contrast
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
