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
  Activity,
  BarChart2,
  Globe,
  History,
  Inbox,
  KeyRound,
  Moon,
  Network,
  PanelLeft,
  ScrollText,
  Search,
  Sun,
  Terminal,
  Users,
  Webhook,
} from 'lucide-react';
import * as React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { AppBarContext } from './contexts/AppBarContext';
import { useUserPreferences } from './contexts/UserPreference';

// Navigation Item Props
type NavItemProps = {
  to: string;
  icon: React.ReactNode;
  text: string;
  isOpen: boolean;
  onClick?: () => void;
};

// Main List Items Props
type MainListItemsProps = {
  isOpen?: boolean;
  onNavItemClick?: () => void;
  onToggle?: () => void;
  customColor?: boolean;
};

// NavItem component with Obsidian Deep styling
function NavItem({ to, icon, text, isOpen, onClick }: NavItemProps) {
  const location = useLocation();
  const isActive = location.pathname.startsWith(to);

  return (
    <div className={cn('px-1', !isOpen && 'flex justify-center')}>
      <Link
        to={to}
        onClick={onClick}
        className={cn(
          'flex items-center rounded-lg transition-all duration-200 ease-in-out px-2 group relative',
          isOpen ? 'h-9 w-full gap-3' : 'h-10 w-10 justify-center',
          isActive
            ? 'text-foreground bg-primary/10 shadow-[inset_0_0_0_1px_rgba(99,102,241,0.2)]'
            : 'text-sidebar-foreground hover:text-foreground hover:bg-white/5'
        )}
        aria-current={isActive ? 'page' : undefined}
        title={isOpen ? '' : text}
      >
        {isActive && (
          <div className="absolute left-0 w-1 h-4 bg-primary rounded-r-full shadow-[0_0_8px_rgba(99,102,241,0.6)]" />
        )}
        <div
          className={cn(
            'transition-transform duration-200 flex items-center justify-center',
            isActive
              ? 'text-primary scale-110'
              : 'opacity-70 group-hover:opacity-100 group-hover:scale-105'
          )}
        >
          {icon}
        </div>
        {isOpen && (
          <span
            className={cn(
              'text-[13px] font-medium transition-colors duration-200 whitespace-nowrap overflow-hidden text-ellipsis',
              isActive
                ? 'text-foreground'
                : 'text-sidebar-foreground group-hover:text-foreground'
            )}
          >
            {text}
          </span>
        )}
      </Link>
    </div>
  );
}

// Exported Nav Items Component
export const mainListItems = React.forwardRef<
  HTMLDivElement,
  MainListItemsProps
>(({ isOpen = false, onNavItemClick, onToggle }, ref) => {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const { preferences, updatePreference } = useUserPreferences();

  const theme = preferences.theme || 'dark';
  const toggleTheme = () => {
    updatePreference('theme', theme === 'dark' ? 'light' : 'dark');
  };

  return (
    <div ref={ref} className="flex flex-col h-full">
      {/* Sidebar Header */}
      <div className="h-14 relative mb-6 flex items-center px-3 border-b border-white/[0.03]">
        {isOpen ? (
          <>
            <div className="flex-1 flex items-center gap-2 truncate">
              <div className="w-7 h-7 bg-primary rounded-lg flex items-center justify-center shadow-[0_0_15px_rgba(var(--primary),0.4)]">
                <span className="text-white font-bold text-sm">D</span>
              </div>
              <span className="font-bold tracking-tight text-lg text-foreground select-none truncate">
                {config.title || 'Dagu'}
              </span>
            </div>
            <button
              onClick={onToggle}
              className="p-1.5 text-muted-foreground hover:text-foreground hover:bg-white/5 rounded-lg transition-all"
              aria-label="Collapse sidebar"
            >
              <PanelLeft size={18} />
            </button>
          </>
        ) : (
          <button
            onClick={onToggle}
            className="mx-auto w-10 h-10 flex items-center justify-center rounded-xl bg-white/5 hover:bg-white/10 transition-all text-foreground glow-sm active:scale-95"
            aria-label="Expand sidebar"
          >
            <div className="w-7 h-7 bg-primary rounded-lg flex items-center justify-center shadow-[0_0_15px_rgba(var(--primary),0.4)]">
              <span className="text-white font-bold text-sm">D</span>
            </div>
          </button>
        )}
      </div>

      <nav className="flex-1 flex flex-col gap-6">
        {/* Remote Node Selector */}
        <AppBarContext.Consumer>
          {(context) => {
            if (!context.remoteNodes || context.remoteNodes.length === 0)
              return null;
            return (
              <div className="px-1">
                {isOpen ? (
                  <Select
                    value={context.selectedRemoteNode}
                    onValueChange={context.selectRemoteNode}
                  >
                    <SelectTrigger className="h-9 w-full bg-white/5 border-white/5 text-xs text-sidebar-foreground hover:bg-white/10 transition-colors">
                      <div className="flex items-center gap-2 truncate">
                        <Globe size={14} className="opacity-60" />
                        <SelectValue />
                      </div>
                    </SelectTrigger>
                    <SelectContent>
                      {context.remoteNodes.map((node) => (
                        <SelectItem key={node} value={node}>
                          {node}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Select
                    value={context.selectedRemoteNode}
                    onValueChange={context.selectRemoteNode}
                  >
                    <SelectTrigger className="w-10 h-10 p-0 bg-white/5 border-white/5 hover:bg-white/10 [&>svg:last-child]:hidden flex items-center justify-center rounded-lg transition-all">
                      <Globe size={18} className="text-primary" />
                    </SelectTrigger>
                    <SelectContent>
                      {context.remoteNodes.map((node) => (
                        <SelectItem key={node} value={node}>
                          {node}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
            );
          }}
        </AppBarContext.Consumer>

        {/* Nav Sections */}
        <div className="space-y-6">
          <div className="space-y-1">
            {isOpen && (
              <div className="px-3 mb-1 text-[10px] font-bold text-muted-foreground/50 uppercase tracking-widest">
                System
              </div>
            )}
            <NavItem
              to="/dashboard"
              text="Dashboard"
              icon={<BarChart2 size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
            />
            {isAdmin && (
              <NavItem
                to="/system-status"
                text="System Status"
                icon={<Activity size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
              />
            )}
          </div>

          <div className="space-y-1">
            {isOpen && (
              <div className="px-3 mb-1 text-[10px] font-bold text-muted-foreground/50 uppercase tracking-widest">
                Workflows
              </div>
            )}
            <NavItem
              to="/queues"
              text="Queues"
              icon={<Inbox size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
            />
            <NavItem
              to="/dag-runs"
              text="Runs"
              icon={<History size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
            />
            <NavItem
              to="/dags"
              text="Definitions"
              icon={<Network size={18} />}
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
          </div>

          {isAdmin && config.authMode === 'builtin' && (
            <div className="space-y-1">
              {isOpen && (
                <div className="px-3 mb-1 text-[10px] font-bold text-muted-foreground/50 uppercase tracking-widest">
                  Admin
                </div>
              )}
              <NavItem
                to="/users"
                text="Users"
                icon={<Users size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
              />
              <NavItem
                to="/api-keys"
                text="API Keys"
                icon={<KeyRound size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
              />
              <NavItem
                to="/webhooks"
                text="Webhooks"
                icon={<Webhook size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
              />
              {config.terminalEnabled && (
                <NavItem
                  to="/terminal"
                  text="Terminal"
                  icon={<Terminal size={18} />}
                  isOpen={isOpen}
                  onClick={onNavItemClick}
                />
              )}
              <NavItem
                to="/audit-logs"
                text="Audit Logs"
                icon={<ScrollText size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
              />
            </div>
          )}
        </div>
      </nav>

      {/* User & Version & Theme */}
      <div className="mt-auto pt-4 flex flex-col gap-3">
        <div
          className={cn('px-2', !isOpen && 'flex flex-col items-center gap-2')}
        >
          <button
            onClick={toggleTheme}
            className={cn(
              'flex items-center gap-3 w-full p-2 rounded-lg transition-all duration-200 hover:bg-white/5 group',
              !isOpen && 'justify-center border border-white/5'
            )}
            title={
              isOpen
                ? ''
                : `Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`
            }
          >
            <div className="flex items-center justify-center text-primary group-hover:scale-110 transition-transform">
              {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
            </div>
            {isOpen && (
              <span className="text-[13px] font-medium text-sidebar-foreground group-hover:text-foreground">
                {theme === 'dark' ? 'Light Mode' : 'Dark Mode'}
              </span>
            )}
          </button>
          <UserMenu isCollapsed={!isOpen} />
        </div>
        {config.version && (
          <div
            className={cn(
              'px-4 pb-4 text-[10px] font-mono text-muted-foreground/70',
              !isOpen && 'text-center px-0'
            )}
          >
            {isOpen ? `v${config.version}` : config.version.split('.')[0]}
          </div>
        )}
      </div>
    </div>
  );
});
mainListItems.displayName = 'MainListItems';
