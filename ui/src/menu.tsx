import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { UserMenu } from '@/components/UserMenu';
import {
  useCanAccessSystemStatus,
  useCanManageWebhooks,
  useCanViewAuditLogs,
  useCanWrite,
  useIsAdmin,
} from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils';
import { getResponsiveTitleClass } from '@/lib/text-utils';
import {
  Activity,
  BarChart2,
  Bot,
  Brain,
  FileCog,
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
  customColor?: boolean;
};

// GCP-Style Section Labels - Subtle & Professional
function SectionLabel({ label, isOpen, customColor = false }: SectionLabelProps): React.ReactElement {
  return (
    <div
      className={cn(
        'px-3 mb-2 mt-1 text-[11px] font-medium uppercase tracking-wide overflow-hidden whitespace-nowrap',
        customColor ? 'text-sidebar-foreground' : 'text-sidebar-foreground/60'
      )}
      style={{
        transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), max-height 250ms cubic-bezier(0.4, 0, 0.2, 1)',
        opacity: isOpen ? 1 : 0,
        maxHeight: isOpen ? '24px' : '0px',
      }}
    >
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
      className="flex items-center gap-3 w-full p-2 rounded-md hover:bg-sidebar-hover group focus-visible:ring-1 focus-visible:ring-ring"
      style={{ transition: 'background-color 150ms ease' }}
      title={isOpen ? '' : label}
    >
      <div className="flex items-center justify-center flex-shrink-0 text-sidebar-foreground group-hover:text-foreground">
        {icon}
      </div>
      <span
        className="text-sm font-medium text-sidebar-foreground group-hover:text-foreground whitespace-nowrap overflow-hidden"
        style={{
          transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), transform 200ms cubic-bezier(0.4, 0, 0.2, 1), max-width 280ms cubic-bezier(0.4, 0, 0.2, 1)',
          opacity: isOpen ? 1 : 0,
          maxWidth: isOpen ? '180px' : '0px',
          transform: isOpen ? 'translateX(0)' : 'translateX(-8px)',
        }}
      >
        {label}
      </span>
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
    'flex items-center rounded-md px-2 group relative',
    'h-9 gap-3',
    isActive
      ? getActiveLinkStyle(customColor)
      : 'text-sidebar-foreground hover:text-foreground hover:bg-sidebar-hover'
  );

  const iconClassName = cn(
    'flex items-center justify-center flex-shrink-0',
    isActive
      ? getActiveIconStyle(customColor)
      : 'text-sidebar-foreground group-hover:text-foreground'
  );

  return (
    <div className="px-1">
      <Link
        to={to}
        onClick={onClick}
        className={linkClassName}
        aria-current={isActive ? 'page' : undefined}
        title={isOpen ? '' : text}
        style={{ transition: 'background-color 150ms ease, color 150ms ease' }}
      >
        {isActive && (
          <div
            className={cn(
              'absolute left-0 w-[3px] h-6 rounded-r-sm',
              getActiveIndicatorStyle(customColor)
            )}
            style={{ transition: 'opacity 200ms ease' }}
          />
        )}
        <div className={iconClassName}>
          {icon}
        </div>
        <span
          className={cn(
            'text-sm font-medium whitespace-nowrap overflow-hidden',
            isActive
              ? 'text-foreground'
              : 'text-sidebar-foreground group-hover:text-foreground'
          )}
          style={{
            transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), transform 200ms cubic-bezier(0.4, 0, 0.2, 1), max-width 280ms cubic-bezier(0.4, 0, 0.2, 1)',
            opacity: isOpen ? 1 : 0,
            maxWidth: isOpen ? '180px' : '0px',
            transform: isOpen ? 'translateX(0)' : 'translateX(-8px)',
          }}
        >
          {text}
        </span>
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
  const canAccessSystemStatus = useCanAccessSystemStatus();
  const canManageWebhooks = useCanManageWebhooks();
  const canViewAuditLogs = useCanViewAuditLogs();
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
        className="h-14 relative mb-4 flex items-center border-b border-sidebar-border px-1"
      >
        <button
          onClick={onToggle}
          className={cn(
            'h-9 px-2 rounded-md flex-shrink-0 flex items-center justify-center',
            customColor
              ? 'hover:opacity-70'
              : 'text-sidebar-foreground hover:text-foreground hover:bg-sidebar-hover'
          )}
          style={{ transition: 'background-color 150ms ease' }}
          aria-label={isOpen ? 'Collapse sidebar' : 'Expand sidebar'}
        >
          {/* Expand icon (character) - visible when collapsed */}
          <div
            className="w-7 h-7 rounded-md flex items-center justify-center border border-sidebar-foreground absolute"
            style={{
              transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), transform 200ms cubic-bezier(0.4, 0, 0.2, 1)',
              opacity: isOpen ? 0 : 1,
              transform: isOpen ? 'scale(0.8)' : 'scale(1)',
              pointerEvents: isOpen ? 'none' : 'auto',
            }}
          >
            <span className="font-medium text-xs text-sidebar-foreground">
              {titleInitial}
            </span>
          </div>
          {/* Collapse icon (PanelLeft) - visible when expanded */}
          <div
            style={{
              transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), transform 200ms cubic-bezier(0.4, 0, 0.2, 1)',
              opacity: isOpen ? 1 : 0,
              transform: isOpen ? 'scale(1)' : 'scale(0.8)',
            }}
          >
            <PanelLeft size={18} />
          </div>
        </button>
        <span
          className={cn(
            'font-semibold tracking-tight text-foreground select-none whitespace-nowrap leading-tight ml-1 overflow-hidden',
            getResponsiveTitleClass(title, 'sidebar-expanded')
          )}
          style={{
            transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), transform 200ms cubic-bezier(0.4, 0, 0.2, 1), max-width 280ms cubic-bezier(0.4, 0, 0.2, 1)',
            opacity: isOpen ? 1 : 0,
            maxWidth: isOpen ? '180px' : '0px',
            transform: isOpen ? 'translateX(0)' : 'translateX(-8px)',
          }}
        >
          {title}
        </span>
      </div>

      {/* GCP-Style Navigation - Compact Spacing */}
      <nav className="flex-1 flex flex-col gap-4">
        <AppBarContext.Consumer>
          {(context) => {
            const { remoteNodes, selectedRemoteNode, selectRemoteNode } = context;
            if (!remoteNodes || remoteNodes.length === 0) return null;

            return (
              <div className="px-1">
                <Select value={selectedRemoteNode} onValueChange={selectRemoteNode}>
                  <SelectTrigger
                    className={cn(
                      'h-9 text-xs text-sidebar-foreground rounded-md',
                      isOpen
                        ? 'bg-sidebar-hover border-sidebar-border hover:bg-sidebar-active'
                        : 'bg-transparent border-transparent hover:bg-sidebar-hover [&>svg:last-child]:hidden'
                    )}
                    style={{
                      transition: 'width 280ms cubic-bezier(0.4, 0, 0.2, 1), background-color 150ms ease, border-color 150ms ease, padding 280ms cubic-bezier(0.4, 0, 0.2, 1)',
                      width: isOpen ? '100%' : '36px',
                      paddingLeft: isOpen ? '12px' : '9px',
                      paddingRight: isOpen ? '12px' : '9px',
                    }}
                  >
                    <div className="flex items-center gap-2">
                      <Globe size={18} className="text-sidebar-foreground flex-shrink-0" />
                      <span
                        className="overflow-hidden whitespace-nowrap"
                        style={{
                          transition: 'opacity 200ms cubic-bezier(0.4, 0, 0.2, 1), max-width 280ms cubic-bezier(0.4, 0, 0.2, 1)',
                          opacity: isOpen ? 1 : 0,
                          maxWidth: isOpen ? '150px' : '0px',
                        }}
                      >
                        <SelectValue />
                      </span>
                    </div>
                  </SelectTrigger>
                  <RemoteNodeSelectContent nodes={remoteNodes} />
                </Select>
              </div>
            );
          }}
        </AppBarContext.Consumer>

        <div className="space-y-4">
          <div className="space-y-0.5">
            <SectionLabel label="System" isOpen={isOpen} customColor={customColor} />
            <NavItem
              to="/dashboard"
              text="Dashboard"
              icon={<BarChart2 size={18} />}
              isOpen={isOpen}
              onClick={onNavItemClick}
              customColor={customColor}
            />
            {canAccessSystemStatus && (
              <NavItem
                to="/system-status"
                text="System Status"
                icon={<Activity size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
            )}
            {canWrite && (
              <NavItem
                to="/base-config"
                text="Base Config"
                icon={<FileCog size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
            )}
          </div>

          <div className="space-y-0.5">
            <SectionLabel label="Workflows" isOpen={isOpen} customColor={customColor} />
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

          {isAdmin && (
            <div className="space-y-0.5">
              <SectionLabel label="Admin" isOpen={isOpen} customColor={customColor} />
              {config.authMode === 'builtin' && (
                <NavItem
                  to="/users"
                  text="Users"
                  icon={<Users size={18} />}
                  isOpen={isOpen}
                  onClick={onNavItemClick}
                  customColor={customColor}
                />
              )}
              {config.authMode === 'builtin' && (
                <NavItem
                  to="/api-keys"
                  text="API Keys"
                  icon={<KeyRound size={18} />}
                  isOpen={isOpen}
                  onClick={onNavItemClick}
                  customColor={customColor}
                />
              )}
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
                to="/agent-settings"
                text="Agent Settings"
                icon={<Bot size={18} />}
                isOpen={isOpen}
                onClick={onNavItemClick}
                customColor={customColor}
              />
              {config.agentEnabled && (
                <NavItem
                  to="/agent-memory"
                  text="Agent Memory"
                  icon={<Brain size={18} />}
                  isOpen={isOpen}
                  onClick={onNavItemClick}
                  customColor={customColor}
                />
              )}
            </div>
          )}

          {(canManageWebhooks || canViewAuditLogs) && (
              <div className="space-y-0.5">
                <SectionLabel label="Operations" isOpen={isOpen} customColor={customColor} />
                {canManageWebhooks && (
                  <NavItem
                    to="/webhooks"
                    text="Webhooks"
                    icon={<Webhook size={18} />}
                    isOpen={isOpen}
                    onClick={onNavItemClick}
                    customColor={customColor}
                  />
                )}
                {canViewAuditLogs && (
                  <NavItem
                    to="/audit-logs"
                    text="Audit Logs"
                    icon={<ScrollText size={18} />}
                    isOpen={isOpen}
                    onClick={onNavItemClick}
                    customColor={customColor}
                  />
                )}
              </div>
            )}

          {canWrite && config.gitSyncEnabled && (
            <div className="space-y-0.5">
              <SectionLabel label="Sync" isOpen={isOpen} customColor={customColor} />
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
