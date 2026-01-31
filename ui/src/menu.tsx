import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { UserMenu } from '@/components/UserMenu';
import { useIsAdmin, useCanWrite } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils';
import { getResponsiveTitleClass } from '@/lib/text-utils';
import {
  Activity,
  BarChart2,
  Bot,
  GitBranch,
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
import { useAgentChatContext } from './features/agent';

type NavItemProps = {
  to: string;
  icon: React.ReactNode;
  text: string;
  isOpen: boolean;
  onClick?: () => void;
  customColor?: boolean;
};

type MainListItemsProps = {
  isOpen?: boolean;
  onNavItemClick?: () => void;
  onToggle?: () => void;
  customColor?: boolean;
};

const DEFAULT_TITLE = 'Dagu';

function getTitleInitial(title: string): string {
  return title.charAt(0).toUpperCase();
}

function getActiveIndicatorStyle(customColor: boolean): string {
  return customColor
    ? 'bg-white shadow-[0_0_8px_rgba(255,255,255,0.4)]'
    : 'bg-primary shadow-[0_0_8px_var(--primary)]';
}

function getActiveLinkStyle(customColor: boolean): string {
  return customColor
    ? 'text-foreground bg-white/10 shadow-[0_0_15px_rgba(255,255,255,0.1),inset_0_0_0_1px_rgba(255,255,255,0.2)]'
    : 'text-foreground bg-primary/10 shadow-[0_0_15px_rgba(var(--primary-rgb),0.15),inset_0_0_0_1px_rgba(var(--primary-rgb),0.3)]';
}

function getActiveIconStyle(customColor: boolean): string {
  return customColor ? 'text-white scale-110' : 'text-primary scale-110';
}

function getIconWrapperStyle(customColor: boolean): string {
  return customColor ? 'opacity-80' : 'text-primary';
}

type RemoteNodeSelectContentProps = {
  nodes: string[];
};

function RemoteNodeSelectContent({ nodes }: RemoteNodeSelectContentProps): React.ReactElement {
  return (
    <SelectContent>
      {nodes.map((node) => (
        <SelectItem key={node} value={node}>
          {node}
        </SelectItem>
      ))}
    </SelectContent>
  );
}

type SectionLabelProps = {
  label: string;
  isOpen: boolean;
};

function SectionLabel({ label, isOpen }: SectionLabelProps): React.ReactElement | null {
  if (!isOpen) return null;

  return (
    <div className="px-3 mb-1 text-xs font-bold text-muted-foreground/50 uppercase tracking-widest">
      {label}
    </div>
  );
}

type SidebarButtonProps = {
  onClick: () => void;
  icon: React.ReactNode;
  label: string;
  isOpen: boolean;
  customColor: boolean;
};

function SidebarButton({ onClick, icon, label, isOpen, customColor }: SidebarButtonProps): React.ReactElement {
  return (
    <button
      onClick={onClick}
      className={cn(
        'flex items-center gap-3 w-full p-2 rounded-lg transition-all duration-200 hover:bg-white/5 group',
        !isOpen && 'justify-center border border-white/5'
      )}
      title={isOpen ? '' : label}
    >
      <div
        className={cn(
          'flex items-center justify-center group-hover:scale-110 transition-transform',
          getIconWrapperStyle(customColor)
        )}
      >
        {icon}
      </div>
      {isOpen && (
        <span className="text-sm font-medium text-sidebar-foreground group-hover:text-foreground">
          {label}
        </span>
      )}
    </button>
  );
}

function NavItem({ to, icon, text, isOpen, onClick, customColor = false }: NavItemProps): React.ReactElement {
  const location = useLocation();
  const isActive =
    location.pathname === to ||
    (to !== '/' && location.pathname.startsWith(to + '/'));

  const linkClassName = cn(
    'flex items-center rounded-lg transition-all duration-200 ease-in-out px-2 group relative',
    isOpen ? 'h-9 w-full gap-3' : 'h-10 w-10 justify-center',
    isActive
      ? getActiveLinkStyle(customColor)
      : 'text-sidebar-foreground hover:text-foreground hover:bg-white/5'
  );

  const iconClassName = cn(
    'transition-transform duration-200 flex items-center justify-center',
    isActive
      ? getActiveIconStyle(customColor)
      : 'opacity-70 group-hover:opacity-100 group-hover:scale-105'
  );

  return (
    <div className={cn('px-1', !isOpen && 'flex justify-center')}>
      <Link
        to={to}
        onClick={onClick}
        className={linkClassName}
        aria-current={isActive ? 'page' : undefined}
        title={isOpen ? '' : text}
      >
        {isActive && (
          <div className={cn(
            'absolute left-0 w-1 h-4 rounded-r-full',
            getActiveIndicatorStyle(customColor)
          )} />
        )}
        <div className={iconClassName}>
          {icon}
        </div>
        {isOpen && (
          <span
            className={cn(
              'text-sm font-medium transition-colors duration-200 whitespace-nowrap overflow-hidden text-ellipsis',
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

export const mainListItems = React.forwardRef<
  HTMLDivElement,
  MainListItemsProps
>(function MainListItems({ isOpen = false, onNavItemClick, onToggle, customColor = false }, ref) {
  const config = useConfig();
  const isAdmin = useIsAdmin();
  const canWrite = useCanWrite();
  const { preferences, updatePreference } = useUserPreferences();
  const { toggleChat } = useAgentChatContext();

  const theme = preferences.theme || 'dark';
  const title = config.title || DEFAULT_TITLE;
  const titleInitial = getTitleInitial(title);

  function toggleTheme(): void {
    updatePreference('theme', theme === 'dark' ? 'light' : 'dark');
  }

  return (
    <div ref={ref} className="flex flex-col h-full">
      <div
        className={cn(
          'h-14 relative mb-6 flex items-center border-b border-white/[0.03]',
          isOpen ? 'px-3' : 'justify-center'
        )}
      >
        {isOpen ? (
          <>
            <div className="flex-1 flex items-center gap-2">
              {!customColor && (
                <div className="w-7 h-7 bg-primary rounded-lg flex-shrink-0 flex items-center justify-center shadow-[0_0_15px_rgba(var(--primary),0.2)]">
                  <span className="text-white font-bold text-sm">
                    {titleInitial}
                  </span>
                </div>
              )}
              <span
                className={cn(
                  'font-bold tracking-tight text-sidebar-foreground select-none whitespace-normal leading-tight',
                  getResponsiveTitleClass(title, 'sidebar-expanded')
                )}
              >
                {title}
              </span>
            </div>
            <button
              onClick={onToggle}
              className="p-1.5 text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-white/5 rounded-lg transition-all"
              aria-label="Collapse sidebar"
            >
              <PanelLeft size={18} />
            </button>
          </>
        ) : (
          <button
            onClick={onToggle}
            className={cn(
              'w-10 h-10 flex items-center justify-center rounded-xl transition-all active:scale-95',
              !customColor &&
                'bg-white/5 hover:bg-white/10 text-foreground glow-sm'
            )}
            aria-label="Expand sidebar"
          >
            {customColor ? (
              <span className="text-xl font-bold text-sidebar-foreground">
                {titleInitial}
              </span>
            ) : (
              <div className="w-7 h-7 bg-primary rounded-lg flex items-center justify-center shadow-[0_0_15px_rgba(var(--primary),0.2)]">
                <span className="text-white font-bold text-sm">
                  {titleInitial}
                </span>
              </div>
            )}
          </button>
        )}
      </div>

      <nav className="flex-1 flex flex-col gap-6">
        <AppBarContext.Consumer>
          {(context) => {
            const { remoteNodes, selectedRemoteNode, selectRemoteNode } = context;
            if (!remoteNodes || remoteNodes.length === 0) return null;

            return (
              <div className={cn('px-1', !isOpen && 'flex justify-center')}>
                {isOpen ? (
                  <Select value={selectedRemoteNode} onValueChange={selectRemoteNode}>
                    <SelectTrigger className="h-9 w-full bg-white/5 border-white/5 text-xs text-sidebar-foreground hover:bg-white/10 transition-colors">
                      <div className="flex items-center gap-2 truncate">
                        <Globe size={14} className="opacity-60" />
                        <SelectValue />
                      </div>
                    </SelectTrigger>
                    <RemoteNodeSelectContent nodes={remoteNodes} />
                  </Select>
                ) : (
                  <Select value={selectedRemoteNode} onValueChange={selectRemoteNode}>
                    <SelectTrigger className="w-10 h-10 p-0 bg-white/5 border-white/5 hover:bg-white/10 [&>svg:last-child]:hidden flex items-center justify-center rounded-lg transition-all">
                      <Globe
                        size={18}
                        className={getIconWrapperStyle(customColor)}
                      />
                    </SelectTrigger>
                    <RemoteNodeSelectContent nodes={remoteNodes} />
                  </Select>
                )}
              </div>
            );
          }}
        </AppBarContext.Consumer>

        <div className="space-y-6">
          <div className="space-y-1">
            <SectionLabel label="System" isOpen={isOpen} />
            <NavItem
              to="/dashboard"
              text="Dashboard"
              icon={<BarChart2 size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
              customColor={customColor}
            />
            {isAdmin && (
              <NavItem
                to="/system-status"
                text="System Status"
                icon={<Activity size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
            )}
          </div>

          <div className="space-y-1">
            <SectionLabel label="Workflows" isOpen={isOpen} />
            <NavItem
              to="/queues"
              text="Queues"
              icon={<Inbox size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
              customColor={customColor}
            />
            <NavItem
              to="/dag-runs"
              text="Runs"
              icon={<History size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
              customColor={customColor}
            />
            <NavItem
              to="/dags"
              text="Definitions"
              icon={<Network size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
              customColor={customColor}
            />
            <NavItem
              to="/search"
              text="Search"
              icon={<Search size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
              customColor={customColor}
            />
          </div>

          {isAdmin && config.authMode === 'builtin' && (
            <div className="space-y-1">
              <SectionLabel label="Admin" isOpen={isOpen} />
              <NavItem
                to="/users"
                text="Users"
                icon={<Users size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
              <NavItem
                to="/api-keys"
                text="API Keys"
                icon={<KeyRound size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
              <NavItem
                to="/webhooks"
                text="Webhooks"
                icon={<Webhook size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
              {config.terminalEnabled && (
                <NavItem
                  to="/terminal"
                  text="Terminal"
                  icon={<Terminal size={18} />}
                  isOpen={isOpen}
                  onClick={onNavItemClick}
                  customColor={customColor}
                />
              )}
              <NavItem
                to="/audit-logs"
                text="Audit Logs"
                icon={<ScrollText size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
              <NavItem
                to="/agent-settings"
                text="Agent Settings"
                icon={<Bot size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
            </div>
          )}

          {canWrite && config.gitSyncEnabled && (
            <div className="space-y-1">
              <SectionLabel label="Sync" isOpen={isOpen} />
              <NavItem
                to="/git-sync"
                text="Git Sync"
                icon={<GitBranch size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
            </div>
          )}
        </div>
      </nav>

      <div className="mt-auto pt-4 flex flex-col gap-3">
        <div
          className={cn('px-2', !isOpen && 'flex flex-col items-center gap-2')}
        >
          {config.agentEnabled && (
            <SidebarButton
              onClick={toggleChat}
              icon={<Terminal size={18} />}
              label="Agent"
              isOpen={isOpen}
              customColor={customColor}
            />
          )}
          <SidebarButton
            onClick={toggleTheme}
            icon={theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
            label={theme === 'dark' ? 'Light Mode' : 'Dark Mode'}
            isOpen={isOpen}
            customColor={customColor}
          />
          <UserMenu isCollapsed={!isOpen} />
        </div>
        {config.version && (
          <div
            className={cn(
              'px-4 pb-4 text-xs font-mono text-muted-foreground/70',
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
