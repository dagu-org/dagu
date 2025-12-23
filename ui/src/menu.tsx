import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { UserMenu } from '@/components/UserMenu';
import { useIsAdmin } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils';
import {
  BarChart2,
  Globe,
  History,
  Inbox,
  Network,
  PanelLeft,
  Search,
  Users,
} from 'lucide-react';
import * as React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { AppBarContext } from './contexts/AppBarContext';

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
        'flex items-center justify-center w-4 h-4 transform-none text-sidebar-foreground',
        isActive ? 'text-sidebar-foreground' : 'text-sidebar-foreground'
      )}
    >
      {children}
    </span>
  );
}

// Define props for mainListItems to accept isOpen, onNavItemClick, onToggle, and customColor
type MainListItemsProps = {
  isOpen?: boolean;
  onNavItemClick?: () => void;
  onToggle?: () => void;
  customColor?: boolean;
};

// Main navigation items structure - now accepts isOpen prop
export const mainListItems = React.forwardRef<
  HTMLDivElement,
  MainListItemsProps
>(({ isOpen = false, onNavItemClick, onToggle }, ref) => {
  // Get version from config at the top level of the component
  const config = useConfig();
  const isAdmin = useIsAdmin();

  // State for hover
  const [isHovered, setIsHovered] = React.useState(false);

  return (
    <div ref={ref} className="flex flex-col h-full">
      {/* Fixed height header with menu toggle button */}
      <div className="h-8 relative mb-1">
        {/* When collapsed: Show first letter of title, switch to panel icon on hover */}
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
                className="text-sidebar-foreground hover:text-sidebar-foreground/70"
              />
            ) : (
              <span className="text-lg font-bold text-sidebar-foreground">
                {(config.title || 'Dagu').charAt(0).toUpperCase()}
              </span>
            )}
          </button>
        )}

        {/* When expanded: Title and panel icon on right */}
        {isOpen && (
          <>
            <div className="absolute inset-0 flex items-center pl-3">
              <span className="font-bold tracking-wide select-none text-xl text-sidebar-foreground">
                {config.title || 'Dagu'}
              </span>
            </div>
            <button
              onClick={() => {
                setIsHovered(false);
                onToggle?.();
              }}
              className="absolute right-3 top-1/2 transform -translate-y-1/2 w-6 h-6 flex items-center justify-center z-10 text-sidebar-foreground/40 hover:text-sidebar-foreground/70 transition-all duration-200 cursor-pointer"
              aria-label="Toggle sidebar"
            >
              <PanelLeft size={20} />
            </button>
          </>
        )}
      </div>
      {/* Navigation */}
      <nav className="flex-1 flex flex-col py-1 px-1">
        {/* Remote Node Selector */}
        <AppBarContext.Consumer>
          {(context) => {
            // Hide remote node selector when builtin auth is enabled or no remote nodes
            if (
              config.authMode === 'builtin' ||
              !context.remoteNodes ||
              context.remoteNodes.length === 0
            ) {
              return null;
            }

            return (
              <div className="mb-3 px-1">
                {isOpen ? (
                  <Select
                    value={context.selectedRemoteNode}
                    onValueChange={context.selectRemoteNode}
                  >
                    <SelectTrigger className="h-8 w-full text-xs bg-sidebar-foreground/10 border border-sidebar-foreground/20 text-sidebar-foreground hover:bg-sidebar-foreground/15 focus:ring-1 focus:ring-sidebar-foreground/30 focus:ring-offset-0 rounded-md">
                      <div className="flex items-center gap-2">
                        <Globe size={14} className="shrink-0 opacity-70" />
                        <SelectValue />
                      </div>
                    </SelectTrigger>
                    <SelectContent>
                      {context.remoteNodes.map((node) => (
                        <SelectItem key={node} value={node} className="text-sm">
                          {node}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <div className="flex justify-center">
                    <Select
                      value={context.selectedRemoteNode}
                      onValueChange={context.selectRemoteNode}
                    >
                      <SelectTrigger
                        className="w-8 h-8 p-0 bg-sidebar-foreground/10 border border-sidebar-foreground/20 text-sidebar-foreground hover:bg-sidebar-foreground/15 focus:ring-1 focus:ring-sidebar-foreground/30 focus:ring-offset-0 rounded-md [&>svg:last-child]:hidden flex items-center justify-center"
                        title={`Node: ${context.selectedRemoteNode}`}
                      >
                        <Globe size={16} className="shrink-0" />
                      </SelectTrigger>
                      <SelectContent>
                        {context.remoteNodes.map((node) => (
                          <SelectItem key={node} value={node} className="text-sm">
                            {node}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </div>
            );
          }}
        </AppBarContext.Consumer>

        {/* Overview Section */}
        <div className="space-y-0.5">
          {isOpen && (
            <div className="px-2 py-0.5">
              <span className="text-[11px] text-sidebar-foreground/70 font-medium">
                Overview
              </span>
            </div>
          )}
          <NavItem
            to="/dashboard"
            text="Dashboard"
            icon={<BarChart2 size={16} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
        </div>

        {/* Workflows Section */}
        <div className="space-y-0.5 mt-2">
          {!isOpen && (
            <div className="mx-auto w-4 border-t border-sidebar-foreground/30 mb-1" />
          )}
          {isOpen && (
            <div className="px-2 py-0.5">
              <span className="text-[11px] text-sidebar-foreground/70 font-medium">
                Workflows
              </span>
            </div>
          )}
          <NavItem
            to="/queues"
            text="Queues"
            icon={<Inbox size={16} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
          <NavItem
            to="/dag-runs"
            text="DAG Runs"
            icon={<History size={16} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
          <NavItem
            to="/dags"
            text="DAG Definitions"
            icon={<Network size={16} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
          <NavItem
            to="/search"
            text="Search DAG Definitions"
            icon={<Search size={16} />}
            isOpen={isOpen}
            onClick={onNavItemClick}
          />
        </div>

        {/* System Section - only show if admin with builtin auth */}
        {isAdmin && config.authMode === 'builtin' && (
          <div className="space-y-0.5 mt-2">
            {!isOpen && (
              <div className="mx-auto w-4 border-t border-sidebar-foreground/30 mb-1" />
            )}
            {isOpen && (
              <div className="px-2 py-0.5">
                <span className="text-[11px] text-sidebar-foreground/70 font-medium">
                  System
                </span>
              </div>
            )}
            <NavItem
              to="/users"
              text="User Management"
              icon={<Users size={16} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
            />
          </div>
        )}
      </nav>
      {/* User Menu */}
      <div className={cn('px-1', isOpen ? '' : 'flex justify-center')}>
        <UserMenu isCollapsed={!isOpen} />
      </div>
      {/* Version tag at the bottom */}
      {config.version && (
        <div
          className={cn(
            'px-2 pb-2 pt-1 text-[10px] text-sidebar-foreground/50',
            isOpen ? 'text-left' : 'text-center'
          )}
        >
          {isOpen ? config.version : config.version.split('.')[0]}
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
      <Link
        to={to}
        onClick={onClick}
        className={cn(
          'flex items-center h-7 text-xs font-medium rounded transition-all duration-200 ease-in-out px-2 gap-2',
          isActive
            ? 'text-sidebar-foreground bg-sidebar-foreground/10'
            : 'text-sidebar-foreground hover:bg-sidebar-foreground/5'
        )}
        aria-current={isActive ? 'page' : undefined}
        title={text}
      >
        <Icon isActive={isActive}>{icon}</Icon>
        <span className="font-medium text-sidebar-foreground text-xs">
          {text}
        </span>
      </Link>
    );
  } else {
    return (
      <div className="flex justify-center">
        <Link
          to={to}
          onClick={onClick}
          className={cn(
            'flex items-center justify-center w-7 h-7 text-xs font-medium rounded transition-all duration-200 ease-in-out',
            isActive
              ? 'text-sidebar-foreground bg-sidebar-foreground/10'
              : 'text-sidebar-foreground hover:bg-sidebar-foreground/5'
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
