import { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth, hasRole } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';

type ProtectedRouteProps = {
  children: ReactNode;
  requiredRole?: 'admin' | 'manager' | 'operator' | 'viewer';
};

/**
 * Renders `children` only when built-in authentication and optional role checks permit access; otherwise performs the appropriate redirect or renders nothing while auth state is loading.
 *
 * If `config.authMode` is not `'builtin'`, access is allowed and `children` are rendered. While auth state is loading the component renders `null`. If the user is not authenticated it redirects to `/login` and preserves the current location for post-login navigation. If a `requiredRole` is provided and the authenticated user lacks that role it redirects to `/`.
 *
 * @param requiredRole - Optional role required to access the route; one of `'admin' | 'manager' | 'operator' | 'viewer'`.
 * @returns The `children` element when access is allowed, `null` while auth state is loading, or a `Navigate` element that redirects the user when access is denied.
 */
export function ProtectedRoute({ children, requiredRole }: ProtectedRouteProps) {
  const config = useConfig();
  const { isAuthenticated, isLoading, user } = useAuth();
  const location = useLocation();

  // If auth mode is not builtin, allow access
  if (config.authMode !== 'builtin') {
    return <>{children}</>;
  }

  // Show nothing while loading auth state
  if (isLoading) {
    return null;
  }

  // Redirect to login if not authenticated
  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Check role requirement if specified
  if (requiredRole && user && !hasRole(user.role, requiredRole)) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}