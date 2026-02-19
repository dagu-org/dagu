import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { useConfig } from './ConfigContext';

export type UserRole = 'admin' | 'manager' | 'developer' | 'operator' | 'viewer';

type User = {
  id: string;
  username: string;
  role: UserRole;
};

type SetupResult = {
  token: string;
  user: User;
};

type AuthContextType = {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  setupRequired: boolean;
  login: (username: string, password: string) => Promise<void>;
  setup: (username: string, password: string) => Promise<SetupResult>;
  logout: () => void;
  refreshUser: () => Promise<void>;
  completeSetup: (result: SetupResult) => void;
};

const AuthContext = createContext<AuthContextType | null>(null);

export const TOKEN_KEY = 'dagu_auth_token';

const ROLE_HIERARCHY: Record<UserRole, number> = {
  admin: 5,
  manager: 4,
  developer: 3,
  operator: 2,
  viewer: 1,
};

export function AuthProvider({ children }: { children: ReactNode }) {
  const config = useConfig();
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_KEY));
  const [isLoading, setIsLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(config.setupRequired);

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

  const setup = useCallback(async (username: string, password: string): Promise<SetupResult> => {
    const response = await fetch(`${config.apiURL}/auth/setup`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    });

    if (!response.ok) {
      const data = await response.json().catch(() => ({}));
      const err = new Error(data.message || 'Setup failed');
      (err as any).status = response.status;
      throw err;
    }

    const data = await response.json();
    return { token: data.token, user: data.user };
  }, [config.apiURL]);

  const completeSetup = useCallback((result: SetupResult) => {
    localStorage.setItem(TOKEN_KEY, result.token);
    setToken(result.token);
    setUser(result.user);
    setSetupRequired(false);
  }, []);

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
        setupRequired,
        login,
        setup,
        completeSetup,
        logout,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}

export function useIsAdmin(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return true;
  return user?.role === 'admin';
}

export function useCanWrite(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return config.permissions.writeDags;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['developer'];
}

export function useCanExecute(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return config.permissions.runDags;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['operator'];
}

export function useCanAccessSystemStatus(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return true;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['developer'];
}

export function useCanManageWebhooks(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  // Webhooks require the builtin auth service (webhook token store).
  if (config.authMode !== 'builtin') return false;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['developer'];
}

export function useCanViewAuditLogs(): boolean {
  const { user } = useAuth();
  const config = useConfig();
  if (config.authMode !== 'builtin') return true;
  if (!user) return false;
  return ROLE_HIERARCHY[user.role] >= ROLE_HIERARCHY['manager'];
}

export function hasRole(userRole: UserRole, requiredRole: UserRole): boolean {
  return ROLE_HIERARCHY[userRole] >= ROLE_HIERARCHY[requiredRole];
}
