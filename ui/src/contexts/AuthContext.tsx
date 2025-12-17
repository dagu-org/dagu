import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { useConfig } from './ConfigContext';

type UserRole = 'admin' | 'manager' | 'operator' | 'viewer';

type User = {
  id: string;
  username: string;
  role: UserRole;
};

type AuthContextType = {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  refreshUser: () => Promise<void>;
};

const AuthContext = createContext<AuthContextType | null>(null);

export const TOKEN_KEY = 'dagu_auth_token';

// Role hierarchy for permission checking
const ROLE_HIERARCHY: Record<UserRole, number> = {
  admin: 4,
  manager: 3,
  operator: 2,
  viewer: 1,
};

/**
 * Provides authentication state and actions to descendant components.
 *
 * Exposes context values for the current user, auth token, authentication status,
 * loading state, and functions to log in, log out, and refresh the user from the API.
 * The provider persists the auth token to localStorage and respects the configured
 * authentication mode when initializing or refreshing user state.
 *
 * @param children - React nodes that receive the authentication context
 * @returns The context provider element that supplies authentication state and actions
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const config = useConfig();
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY));
  const [isLoading, setIsLoading] = useState(true);

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY);
    setToken(null);
    setUser(null);
  }, []);

  const refreshUser = useCallback(async () => {
    const storedToken = localStorage.getItem(TOKEN_KEY);
    if (!storedToken) {
      setIsLoading(false);
      return;
    }

    try {
      const response = await fetch(`${config.apiURL}/auth/me`, {
        headers: {
          Authorization: `Bearer ${storedToken}`,
        },
      });

      if (response.ok) {
        const data = await response.json();
        setUser(data.user);
        setToken(storedToken);
      } else {
        logout();
      }
    } catch {
      logout();
    } finally {
      setIsLoading(false);
    }
  }, [config.apiURL, logout]);

  const login = useCallback(async (username: string, password: string) => {
    const response = await fetch(`${config.apiURL}/auth/login`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      throw new Error(data.message || 'Login failed');
    }

    const data = await response.json();
    localStorage.setItem(TOKEN_KEY, data.token);
    setToken(data.token);
    setUser(data.user);
  }, [config.apiURL]);

  useEffect(() => {
    if (config.authMode === 'builtin') {
      refreshUser();
    } else {
      setIsLoading(false);
    }
  }, [config.authMode, refreshUser]);

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isAuthenticated: !!user,
        isLoading,
        login,
        logout,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

/**
 * Access the authentication context supplied by the nearest AuthProvider in the React tree.
 *
 * @returns The AuthContext value containing `user`, `token`, `isAuthenticated`, `isLoading`, `login`, `logout`, and `refreshUser`.
 * @throws Error if there is no surrounding AuthProvider
 */
export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

/**
 * Determines whether the current user has administrative privileges.
 *
 * @returns `true` if the app is using a non-'builtin' auth mode or the authenticated user's role is `'admin'`, `false` otherwise.
 */
export function useIsAdmin(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return true;
  return user?.role === 'admin';
}

/**
 * Determines whether the current user is permitted to create or modify DAGs.
 *
 * In non-builtin auth mode this follows the `config.permissions.writeDags` flag.
 * In builtin auth mode the user must exist and have role `manager` or `admin`.
 *
 * @returns `true` if writing DAGs is permitted in the current context, `false` otherwise.
 */
export function useCanWrite(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return config.permissions.writeDags;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['manager'];
}

/**
 * Determine whether the current user is permitted to execute (run) DAGs.
 *
 * In non-builtin auth mode this reflects `config.permissions.runDags`; in builtin mode this requires the user's role to be `operator` or higher.
 *
 * @returns `true` if execution is permitted for the current context, `false` otherwise.
 */
export function useCanExecute(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return config.permissions.runDags;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['operator'];
}

/**
 * Determine whether a user's role meets or exceeds a required role in the role hierarchy.
 *
 * @param userRole - The role held by the user
 * @param requiredRole - The minimum role required
 * @returns `true` if `userRole` has at least the permissions of `requiredRole`, `false` otherwise.
 */
export function hasRole(userRole: UserRole, requiredRole: UserRole): boolean {
  return ROLE_HIERARCHY[userRole] >= ROLE_HIERARCHY[requiredRole];
}