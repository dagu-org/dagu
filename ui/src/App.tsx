import { Theme } from '@radix-ui/themes';
import '@radix-ui/themes/styles.css';
import React from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { SWRConfig } from 'swr';
import { ProtectedRoute } from './components/ProtectedRoute';
import { ErrorModalProvider } from './components/ui/error-modal';
import { ToastProvider } from './components/ui/simple-toast';
import { AppBarContext } from './contexts/AppBarContext';
import { AuthProvider } from './contexts/AuthContext';
import { Config, ConfigContext } from './contexts/ConfigContext';
import { SchemaProvider } from './contexts/SchemaContext';
import { SearchStateProvider } from './contexts/SearchStateContext';
import {
  UserPreferencesProvider,
  useUserPreferences,
} from './contexts/UserPreference';
import Layout from './layouts/Layout';
import fetchJson from './lib/fetchJson';
import Dashboard from './pages';
import APIKeysPage from './pages/api-keys';
import AuditLogsPage from './pages/audit-logs';
import DAGRuns from './pages/dag-runs';
import DAGRunDetails from './pages/dag-runs/dag-run';
import DAGs from './pages/dags';
import DAGDetails from './pages/dags/dag';
import LoginPage from './pages/login';
import Queues from './pages/queues';
import Search from './pages/search';
import SystemStatus from './pages/system-status';
import TerminalPage from './pages/terminal';
import UsersPage from './pages/users';
import WebhooksPage from './pages/webhooks';

type Props = {
  config: Config;
};

/**
 * Root application component that composes providers, routing, and global UI state.
 *
 * Initializes and persists the selected remote node, exposes app bar state and config
 * via context providers, and mounts public (login) and protected routes inside the app layout.
 *
 * @param config - Application configuration (e.g., `basePath`, `remoteNodes`) used to configure routing and available remote nodes.
 * @returns The top-level React element for the application.
 */
/**
 * Inner App component that has access to providers
 */
function AppInner({ config }: Props) {
  const [title, setTitle] = React.useState<string>('');
  const { preferences } = useUserPreferences();
  const theme = preferences.theme || 'dark';

  const remoteNodes = config.remoteNodes
    .split(',')
    .filter(Boolean)
    .map((node) => node.trim());
  if (!remoteNodes.includes('local')) {
    remoteNodes.unshift('local');
  }
  const localStorageKey = 'dagu-selected-remote-node';

  // Read initial value from localStorage or default to 'local'
  const getInitialNode = () => {
    const storedNode = localStorage.getItem(localStorageKey);
    if (storedNode && remoteNodes.includes(storedNode)) {
      return storedNode;
    }
    return 'local';
  };

  const [selectedRemoteNode, setSelectedRemoteNode] =
    React.useState<string>(getInitialNode);

  const handleSelectRemoteNode = (node: string) => {
    if (remoteNodes.includes(node)) {
      setSelectedRemoteNode(node);
      localStorage.setItem(localStorageKey, node);
    } else {
      setSelectedRemoteNode('local');
      localStorage.setItem(localStorageKey, 'local');
    }
  };

  React.useEffect(() => {
    if (!remoteNodes.includes(selectedRemoteNode)) {
      handleSelectRemoteNode('local');
    }
  }, [remoteNodes, selectedRemoteNode]);

  // Effect to apply theme class to html element
  React.useEffect(() => {
    const root = document.documentElement;
    if (theme === 'dark') {
      root.classList.add('dark');
      root.style.backgroundColor = '#020617';
    } else {
      root.classList.remove('dark');
      root.style.backgroundColor = '#ffffff';
    }
  }, [theme]);

  return (
    <Theme
      appearance={theme}
      accentColor="indigo"
      grayColor="slate"
      radius="large"
    >
      <SWRConfig
        value={{
          fetcher: fetchJson,
          onError: (err) => {
            console.error(err);
          },
        }}
      >
        <AppBarContext.Provider
          value={{
            title,
            setTitle,
            remoteNodes,
            selectedRemoteNode,
            selectRemoteNode: handleSelectRemoteNode,
          }}
        >
          <ConfigContext.Provider value={config}>
            <AuthProvider>
              <SearchStateProvider>
                <SchemaProvider>
                  <ErrorModalProvider>
                    <ToastProvider>
                      <BrowserRouter basename={config.basePath}>
                        <Routes>
                          {/* Public route - Login page */}
                          <Route path="/login" element={<LoginPage />} />

                          {/* Protected routes */}
                          <Route
                            path="/*"
                            element={
                              <ProtectedRoute>
                                <Layout navbarColor={config.navbarColor}>
                                  <Routes>
                                    <Route path="/" element={<Dashboard />} />
                                    <Route
                                      path="/dashboard"
                                      element={<Dashboard />}
                                    />
                                    <Route
                                      path="/system-status"
                                      element={
                                        <ProtectedRoute requiredRole="admin">
                                          <SystemStatus />
                                        </ProtectedRoute>
                                      }
                                    />
                                    <Route path="/dags/" element={<DAGs />} />
                                    <Route
                                      path="/dags/:fileName/:tab"
                                      element={<DAGDetails />}
                                    />
                                    <Route
                                      path="/dags/:fileName/"
                                      element={<DAGDetails />}
                                    />
                                    <Route
                                      path="/search/"
                                      element={<Search />}
                                    />
                                    <Route
                                      path="/queues"
                                      element={<Queues />}
                                    />
                                    <Route
                                      path="/dag-runs"
                                      element={<DAGRuns />}
                                    />
                                    <Route
                                      path="/dag-runs/:name/:dagRunId"
                                      element={<DAGRunDetails />}
                                    />
                                    {/* Admin-only routes */}
                                    <Route
                                      path="/users"
                                      element={
                                        <ProtectedRoute requiredRole="admin">
                                          <UsersPage />
                                        </ProtectedRoute>
                                      }
                                    />
                                    <Route
                                      path="/api-keys"
                                      element={
                                        <ProtectedRoute requiredRole="admin">
                                          <APIKeysPage />
                                        </ProtectedRoute>
                                      }
                                    />
                                    <Route
                                      path="/webhooks"
                                      element={
                                        <ProtectedRoute requiredRole="admin">
                                          <WebhooksPage />
                                        </ProtectedRoute>
                                      }
                                    />
                                    <Route
                                      path="/terminal"
                                      element={
                                        <ProtectedRoute requiredRole="admin">
                                          <TerminalPage />
                                        </ProtectedRoute>
                                      }
                                    />
                                    <Route
                                      path="/audit-logs"
                                      element={
                                        <ProtectedRoute requiredRole="admin">
                                          <AuditLogsPage />
                                        </ProtectedRoute>
                                      }
                                    />
                                  </Routes>
                                </Layout>
                              </ProtectedRoute>
                            }
                          />
                        </Routes>
                      </BrowserRouter>
                    </ToastProvider>
                  </ErrorModalProvider>
                </SchemaProvider>
              </SearchStateProvider>
            </AuthProvider>
          </ConfigContext.Provider>
        </AppBarContext.Provider>
      </SWRConfig>
    </Theme>
  );
}

function App({ config }: Props) {
  return (
    <UserPreferencesProvider>
      <AppInner config={config} />
    </UserPreferencesProvider>
  );
}

export default App;
