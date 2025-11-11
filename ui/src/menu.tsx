import logoDark from '@/assets/images/logo_dark.png';
import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils'; // Assuming cn utility is available
import {
  Activity,
  BarChart2,
  GitBranch,
  Github,
  Layers,
  List,
  PanelLeft,
  Search,
  Server,
} from 'lucide-react';
import * as React from 'react';
import { Link, useLocation } from 'react-router-dom';

// Discord SVG Icon component
function DiscordIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="currentColor"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path d="M20.317 4.3698a19.7913 19.7913 0 00-4.8851-1.5152.0741.0741 0 00-.0785.0371c-.211.3753-.4447.8648-.6083 1.2495-1.8447-.2762-3.68-.2762-5.4868 0-.1636-.3933-.4058-.8742-.6177-1.2495a.077.077 0 00-.0785-.037 19.7363 19.7363 0 00-4.8852 1.515.0699.0699 0 00-.0321.0277C.5334 9.0458-.319 13.5799.0992 18.0578a.0824.0824 0 00.0312.0561c2.0528 1.5076 4.0413 2.4228 5.9929 3.0294a.0777.0777 0 00.0842-.0276c.4616-.6304.8731-1.2952 1.226-1.9942a.076.076 0 00-.0416-.1057c-.6528-.2476-1.2743-.5495-1.8722-.8923a.077.077 0 01-.0076-.1277c.1258-.0943.2517-.1923.3718-.2914a.0743.0743 0 01.0776-.0105c3.9278 1.7933 8.18 1.7933 12.0614 0a.0739.0739 0 01.0785.0095c.1202.099.246.1981.3728.2924a.077.077 0 01-.0066.1276 12.2986 12.2986 0 01-1.873.8914.0766.0766 0 00-.0407.1067c.3604.698.7719 1.3628 1.225 1.9932a.076.076 0 00.0842.0286c1.961-.6067 3.9495-1.5219 6.0023-3.0294a.077.077 0 00.0313-.0552c.5004-5.177-.8382-9.6739-3.5485-13.6604a.061.061 0 00-.0312-.0286zM8.02 15.3312c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9555-2.4189 2.157-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.9555 2.4189-2.1569 2.4189zm7.9748 0c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9554-2.4189 2.1569-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.946 2.4189-2.1568 2.4189Z" />
    </svg>
  );
}

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

// Define props for mainListItems to accept isOpen, onNavItemClick, and onToggle
type MainListItemsProps = {
  isOpen?: boolean;
  onNavItemClick?: () => void;
  onToggle?: () => void;
};

// Main navigation items structure - now accepts isOpen prop
export const mainListItems = React.forwardRef<
  HTMLDivElement,
  MainListItemsProps
>(({ isOpen = false, onNavItemClick, onToggle }, ref) => {
  // Get version from config at the top level of the component
  const config = useConfig();

  // State for hover
  const [isHovered, setIsHovered] = React.useState(false);

  return (
    <div ref={ref} className="flex flex-col h-full">
      {/* Fixed height header with menu toggle button */}
      <div className="h-12 relative mb-2">
        {/* When collapsed: Show Dagu icon, switch to panel icon on hover */}
        {!isOpen && (
          <button
            onClick={() => {
              setIsHovered(false);
              onToggle?.();
            }}
            onMouseEnter={() => setIsHovered(true)}
            onMouseLeave={() => setIsHovered(false)}
            className="absolute left-3 top-1/2 transform -translate-y-1/2 w-6 h-6 flex items-center justify-center z-10 transition-all duration-200 cursor-pointer"
            aria-label="Toggle sidebar"
          >
            {isHovered ? (
              <PanelLeft
                size={20}
                className="text-primary-foreground hover:text-primary-foreground/70"
              />
            ) : (
              <img
                src={logoDark}
                alt="Dagu Logo"
                className="w-6 h-6 object-contain"
              />
            )}
          </button>
        )}

        {/* When expanded: Dagu logo on left, panel icon on right */}
        {isOpen && (
          <>
            <div className="absolute left-3 top-1/2 transform -translate-y-1/2 w-6 h-6 flex items-center justify-center z-10">
              <img
                src={logoDark}
                alt="Dagu Logo"
                className="w-6 h-6 object-contain"
              />
            </div>
            <div className="absolute inset-0 flex items-center pl-12">
              <span className="font-bold tracking-wide select-none text-xl text-primary-foreground">
                Dagu
              </span>
            </div>
            <button
              onClick={() => {
                setIsHovered(false);
                onToggle?.();
              }}
              className="absolute right-3 top-1/2 transform -translate-y-1/2 w-6 h-6 flex items-center justify-center z-10 text-primary-foreground/40 hover:text-primary-foreground/70 transition-all duration-200 cursor-pointer"
              aria-label="Toggle sidebar"
            >
              <PanelLeft size={20} />
            </button>
          </>
        )}
      </div>
      {/* Navigation */}
      <nav className="flex-1 flex flex-col py-2 px-2">
        {/* Overview Section */}
        <div className="space-y-1">
          {isOpen && (
            <div className="px-3 py-1">
              <span className="text-[10px] uppercase text-primary-foreground/40 font-medium">
                Overview
              </span>
            </div>
          )}
          <NavItem
            to="/dashboard"
            text="Dashboard"
            icon={<BarChart2 size={18} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
        </div>

        {/* Workflows Section */}
        <div className="space-y-1 mt-4">
          {!isOpen && (
            <div className="mx-auto w-4 border-t border-primary-foreground/30 mb-2" />
          )}
          {isOpen && (
            <div className="px-3 py-1">
              <span className="text-[10px] uppercase text-primary-foreground/40 font-medium">
                Workflows
              </span>
            </div>
          )}
          <NavItem
            to="/queues"
            text="Queues"
            icon={<Layers size={18} />}
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
        </div>

        {/* System Section */}
        <div className="space-y-1 mt-4">
          {!isOpen && (
            <div className="mx-auto w-4 border-t border-primary-foreground/30 mb-2" />
          )}
          {isOpen && (
            <div className="px-3 py-1">
              <span className="text-[10px] uppercase text-primary-foreground/40 font-medium">
                System
              </span>
            </div>
          )}
          <NavItem
            to="/workers"
            text="Workers"
            icon={<Server size={18} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
          <NavItem
            to="/system-status"
            text="System Status"
            icon={<Activity size={18} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
        </div>
      </nav>
      {/* Discord Community link */}
      <div className="px-2 pb-1">
        <a
          href="https://discord.gg/gpahPUjGRk"
          target="_blank"
          rel="noopener noreferrer"
          className={cn(
            'flex items-center transition-all duration-200',
            isOpen
              ? 'h-9 px-3 rounded-lg hover:bg-primary-foreground/5 text-primary-foreground/60 hover:text-primary-foreground justify-start'
              : 'w-8 h-8 rounded-lg hover:bg-primary-foreground/5 text-primary-foreground/60 hover:text-primary-foreground justify-center'
          )}
          title="Discord Community"
        >
          <DiscordIcon />
          {isOpen && <span className="ml-3 text-xs font-medium">Discord</span>}
        </a>
      </div>
      {/* GitHub link */}
      <div className="px-2 pb-6 md:pb-1">
        <a
          href="https://github.com/dagu-org/dagu"
          target="_blank"
          rel="noopener noreferrer"
          className={cn(
            'flex items-center transition-all duration-200',
            isOpen
              ? 'h-9 px-3 rounded-lg hover:bg-primary-foreground/5 text-primary-foreground/60 hover:text-primary-foreground justify-start'
              : 'w-8 h-8 rounded-lg hover:bg-primary-foreground/5 text-primary-foreground/60 hover:text-primary-foreground justify-center'
          )}
          title="GitHub Repository"
        >
          <Github size={18} />
          {isOpen && <span className="ml-3 text-xs font-medium">GitHub</span>}
        </a>
      </div>
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
            'block h-9 flex items-center text-xs font-medium rounded-lg transition-all duration-200 ease-in-out pl-10 pr-3',
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
            'flex items-center justify-center w-8 h-8 text-xs font-medium rounded-lg transition-all duration-200 ease-in-out',
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
