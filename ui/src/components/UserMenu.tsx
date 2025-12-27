import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { User, LogOut, Key } from 'lucide-react';
import { ChangePasswordModal } from './ChangePasswordModal';

type UserMenuProps = {
  isCollapsed?: boolean;
};

/**
 * Renders a user dropdown menu with profile info, a change-password action, and sign-out.
 *
 * The menu is rendered only when built-in authentication is enabled and a user is authenticated.
 *
 * @returns The user menu JSX element when shown, or `null` when authentication is not available.
 */
export function UserMenu({ isCollapsed = false }: UserMenuProps) {
  const { user, logout, isAuthenticated } = useAuth();
  const config = useConfig();
  const navigate = useNavigate();
  const [showChangePassword, setShowChangePassword] = useState(false);

  // Don't show if auth is not builtin or user is not authenticated
  if (config.authMode !== 'builtin' || !isAuthenticated || !user) {
    return null;
  }

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            className={cn(
              'flex items-center transition-all duration-200 cursor-pointer',
              isCollapsed
                ? 'w-7 h-7 justify-center rounded hover:bg-sidebar-foreground/5 text-sidebar-foreground'
                : 'h-7 px-2 rounded hover:bg-sidebar-foreground/5 text-sidebar-foreground justify-start w-full'
            )}
            title={isCollapsed ? user.username : undefined}
          >
            <User className="h-4 w-4 shrink-0" />
            {!isCollapsed && (
              <span className="ml-2 text-xs font-medium truncate">
                {user.username}
              </span>
            )}
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align={isCollapsed ? 'center' : 'end'} side="top" className="w-56">
          <DropdownMenuLabel className="font-normal">
            <div className="flex flex-col space-y-1">
              <p className="text-sm font-medium">{user.username}</p>
              <span className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground capitalize w-fit">
                {user.role}
              </span>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setShowChangePassword(true)}>
            <Key className="h-4 w-4 mr-2" />
            Change Password
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleLogout} className="text-error focus:text-error">
            <LogOut className="h-4 w-4 mr-2" />
            Sign Out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <ChangePasswordModal
        open={showChangePassword}
        onClose={() => setShowChangePassword(false)}
      />
    </>
  );
}