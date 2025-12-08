import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Button } from '@/components/ui/button';
import { User, LogOut, Key, Shield } from 'lucide-react';
import { ChangePasswordModal } from './ChangePasswordModal';

export function UserMenu() {
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

  const getRoleBadgeColor = (role: string) => {
    switch (role) {
      case 'admin':
        return 'bg-red-500/20 text-red-600 dark:text-red-400';
      case 'manager':
        return 'bg-blue-500/20 text-blue-600 dark:text-blue-400';
      case 'operator':
        return 'bg-green-500/20 text-green-600 dark:text-green-400';
      case 'viewer':
        return 'bg-gray-500/20 text-gray-600 dark:text-gray-400';
      default:
        return 'bg-gray-500/20 text-gray-600 dark:text-gray-400';
    }
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-current hover:bg-accent/10"
          >
            <User className="h-4 w-4 mr-1.5" />
            <span className="text-sm">{user.username}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-56">
          <DropdownMenuLabel className="font-normal">
            <div className="flex flex-col space-y-1">
              <p className="text-sm font-medium">{user.username}</p>
              <div className="flex items-center gap-1.5">
                <Shield className="h-3 w-3 text-muted-foreground" />
                <span
                  className={`text-xs px-1.5 py-0.5 rounded-full capitalize ${getRoleBadgeColor(user.role)}`}
                >
                  {user.role}
                </span>
              </div>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={() => setShowChangePassword(true)}>
            <Key className="h-4 w-4 mr-2" />
            Change Password
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleLogout} className="text-red-600 dark:text-red-400 focus:text-red-600 dark:focus:text-red-400">
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
