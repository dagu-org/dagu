import { ReactNode } from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth, hasRole } from '@/contexts/AuthContext';
import { useConfig } from '@/contexts/ConfigContext';

type ProtectedRouteProps = {
  children: ReactNode;
  requiredRole?: 'admin' | 'manager' | 'operator' | 'viewer';
};

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
