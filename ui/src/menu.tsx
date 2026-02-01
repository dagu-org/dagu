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

// GCP-Style Active States - Clean & Minimal
function getActiveIndicatorStyle(customColor: boolean): string {
  return customColor
    ? 'bg-white'
    : 'bg-sidebar-primary';
}

function getActiveLinkStyle(customColor: boolean): string {
  return customColor
    ? 'bg-sidebar-active'
    : 'bg-sidebar-active';
}

function getActiveIconStyle(customColor: boolean): string {
  return customColor ? 'text-foreground' : 'text-sidebar-primary';
}

function getIconWrapperStyle(customColor: boolean): string {
  return customColor ? 'text-sidebar-foreground' : 'text-sidebar-foreground';
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

// GCP-Style Section Labels - Subtle & Professional
function SectionLabel({ label, isOpen }: SectionLabelProps): React.ReactElement | null {
  if (!isOpen) return null;

  return (
    <div className="px-3 mb-2 mt-1 text-[11px] font-medium text-sidebar-foreground/60 uppercase tracking-wide">
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

// GCP-Style Sidebar Button - Clean & Minimal
function SidebarButton({ onClick, icon, label, isOpen, customColor }: SidebarButtonProps): React.ReactElement {
  return (
    <button
      onClick={onClick}
      className={cn(
        'flex items-center gap-3 w-full p-2 rounded-md transition-all duration-150 hover:bg-sidebar-hover group',
        !isOpen && 'justify-center'
      )}
      title={isOpen ? '' : label}
    >
      <div
        className={cn(
          'flex items-center justify-center transition-colors',
          'text-sidebar-foreground group-hover:text-foreground'
        )}
      >
        {icon}
      </div>
      {isOpen && (
        <span className="text-sm font-medium text-sidebar-foreground group-hover:text-foreground transition-colors">
          {label}
        </span>
      )}
    </button>
  );
}

// GCP-Style Navigation Item - Clean Active States with Left Border
function NavItem({ to, icon, text, isOpen, onClick, customColor = false }: NavItemProps): React.ReactElement {
  const location = useLocation();
  const isActive =
    location.pathname === to ||
    (to !== '/' && location.pathname.startsWith(to + '/'));

  const linkClassName = cn(
    'flex items-center rounded-md transition-all duration-150 ease-in-out px-2 group relative',
    isOpen ? 'h-9 w-full gap-3' : 'h-9 w-9 justify-center',
    isActive
      ? getActiveLinkStyle(customColor)
      : 'text-sidebar-foreground hover:text-foreground hover:bg-sidebar-hover'
  );

  const iconClassName = cn(
    'transition-colors duration-150 flex items-center justify-center',
    isActive
      ? getActiveIconStyle(customColor)
      : 'text-sidebar-foreground group-hover:text-foreground'
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
            'absolute left-0 w-[3px] h-6 rounded-r-sm',
            getActiveIndicatorStyle(customColor)
          )} />
        )}
        <div className={iconClassName}>
          {icon}
        </div>
        {isOpen && (
          <span
            className={cn(
              'text-sm font-medium transition-colors duration-150 whitespace-nowrap overflow-hidden text-ellipsis',
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
      {/* GCP-Style Header - Clean & Minimal */}
      <div
        className={cn(
          'h-14 relative mb-4 flex items-center border-b border-sidebar-border',
          isOpen ? 'px-3' : 'justify-center'
        )}
      >
        {isOpen ? (
          <>
            <div className="flex-1 flex items-center gap-2.5">
              {!customColor && (
                <div className="w-8 h-8 bg-sidebar-primary rounded-lg flex-shrink-0 flex items-center justify-center">
                  <span className="text-white font-semibold text-sm">
                    {titleInitial}
                  </span>
                </div>
              )}
              <span
                className={cn(
                  'font-semibold tracking-tight text-foreground select-none whitespace-normal leading-tight',
                  getResponsiveTitleClass(title, 'sidebar-expanded')
                )}
              >
                {title}
              </span>
            </div>
            <button
              onClick={onToggle}
              className="p-1.5 text-sidebar-foreground hover:text-foreground hover:bg-sidebar-hover rounded-md transition-all"
              aria-label="Collapse sidebar"
            >
              <PanelLeft size={18} />
            </button>
          </>
        ) : (
          <button
            onClick={onToggle}
            className={cn(
              'w-9 h-9 flex items-center justify-center rounded-lg transition-all',
              !customColor && 'hover:bg-sidebar-hover'
            )}
            aria-label="Expand sidebar"
          >
            {customColor ? (
              <span className="text-lg font-semibold text-sidebar-foreground">
                {titleInitial}
              </span>
            ) : (
              <div className="w-8 h-8 bg-sidebar-primary rounded-lg flex items-center justify-center">
                <span className="text-white font-semibold text-sm">
                  {titleInitial}
                </span>
              </div>
            )}
          </button>
        )}
      </div>

      {/* GCP-Style Navigation - Compact Spacing */}
      <nav className="flex-1 flex flex-col gap-4">
        <AppBarContext.Consumer>
          {(context) => {
            const { remoteNodes, selectedRemoteNode, selectRemoteNode } = context;
            if (!remoteNodes || remoteNodes.length === 0) return null;

            return (
              <div className={cn('px-1', !isOpen && 'flex justify-center')}>
                {isOpen ? (
                  <Select value={selectedRemoteNode} onValueChange={selectRemoteNode}>
                    <SelectTrigger className="h-9 w-full bg-sidebar-hover border-sidebar-border text-xs text-sidebar-foreground hover:bg-sidebar-active transition-colors">
                      <div className="flex items-center gap-2 truncate">
                        <Globe size={14} className="text-sidebar-foreground" />
                        <SelectValue />
                      </div>
                    </SelectTrigger>
                    <RemoteNodeSelectContent nodes={remoteNodes} />
                  </Select>
                ) : (
                  <Select value={selectedRemoteNode} onValueChange={selectRemoteNode}>
                    <SelectTrigger className="w-9 h-9 p-0 bg-transparent border-transparent hover:bg-sidebar-hover [&>svg:last-child]:hidden flex items-center justify-center rounded-md transition-all">
                      <Globe
                        size={18}
                        className="text-sidebar-foreground"
                      />
                    </SelectTrigger>
                    <RemoteNodeSelectContent nodes={remoteNodes} />
                  </Select>
                )}
              </div>
            );
          }}
        </AppBarContext.Consumer>

        <div className="space-y-4">
          <div className="space-y-0.5">
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

          <div className="space-y-0.5">
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
            <div className="space-y-0.5">
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
            <div className="space-y-0.5">
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

      {/* GCP-Style Footer - Clean Controls */}
      <div className="mt-auto pt-3 border-t border-sidebar-border flex flex-col gap-2">
        <div
          className={cn('px-2', !isOpen && 'flex flex-col items-center gap-1.5')}
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
              'px-3 pb-3 text-[11px] font-mono text-sidebar-foreground/50',
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
