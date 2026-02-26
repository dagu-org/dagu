import { Theme } from '@radix-ui/themes';
import '@radix-ui/themes/styles.css';
import React from 'react';
import { BrowserRouter, Link, Route, Routes } from 'react-router-dom';
import { SWRConfig } from 'swr';

import { Shield } from 'lucide-react';

import { ProtectedRoute } from './components/ProtectedRoute';
import { ErrorModalProvider } from './components/ui/error-modal';
import { ToastProvider } from './components/ui/simple-toast';
import { AppBarContext } from './contexts/AppBarContext';
import { AuthProvider } from './contexts/AuthContext';
import { Config, ConfigContext, ConfigUpdateContext } from './contexts/ConfigContext';
import { useHasFeature } from './hooks/useLicense';
import { PageContextProvider } from './contexts/PageContext';
import { SchemaProvider } from './contexts/SchemaContext';
import { SearchStateProvider } from './contexts/SearchStateContext';
import {
  UserPreferencesProvider,
  useUserPreferences,
} from './contexts/UserPreference';
import { AgentChatModal, AgentChatProvider } from './features/agent';
import Layout from './layouts/Layout';
import fetchJson from './lib/fetchJson';
import Dashboard from './pages';
import AgentMemoryPage from './pages/agent-memory';
import AgentSettingsPage from './pages/agent-settings';
import AgentSkillsPage from './pages/agent-skills';
import SkillEditorPage from './pages/agent-skills/SkillEditorPage';
import AgentSoulsPage from './pages/agent-souls';
import SoulEditorPage from './pages/agent-souls/SoulEditorPage';
import APIKeysPage from './pages/api-keys';
import AuditLogsPage from './pages/audit-logs';
import BaseConfigPage from './pages/base-config';
import DAGRuns from './pages/dag-runs';
import DAGRunDetails from './pages/dag-runs/dag-run';
import DAGs from './pages/dags';
import DAGDetails from './pages/dags/dag';
import GitSyncPage from './pages/git-sync';
import LicensePage from './pages/license';
import LoginPage from './pages/login';
import Queues from './pages/queues';
import Search from './pages/search';
import SetupPage from './pages/setup';
import SystemStatus from './pages/system-status';
import TerminalPage from './pages/terminal';
import RemoteNodesPage from './pages/remote-nodes';
import UsersPage from './pages/users';
import WebhooksPage from './pages/webhooks';

type Props = {
  config: Config;
};

const REMOTE_NODE_STORAGE_KEY = 'dagu-selected-remote-node';

function parseRemoteNodes(remoteNodesConfig: string): string[] {
  const nodes = remoteNodesConfig
    .split(',')
    .filter(Boolean)
    .map((node) => node.trim());
  if (!nodes.includes('local')) {
    nodes.unshift('local');
  }
  return nodes;
}

function getStoredRemoteNode(validNodes: string[]): string {
  const storedNode = localStorage.getItem(REMOTE_NODE_STORAGE_KEY);
  if (storedNode && validNodes.includes(storedNode)) {
    return storedNode;
  }
  return 'local';
}

// Helper to wrap admin-only elements
function AdminElement({
  children,
}: {
  children: React.ReactElement;
}): React.ReactElement {
  return <ProtectedRoute requiredRole="admin">{children}</ProtectedRoute>;
}

function ManagerElement({
  children,
}: {
  children: React.ReactElement;
}): React.ReactElement {
  return <ProtectedRoute requiredRole="manager">{children}</ProtectedRoute>;
}

function DeveloperElement({
  children,
}: {
  children: React.ReactElement;
}): React.ReactElement {
  return <ProtectedRoute requiredRole="developer">{children}</ProtectedRoute>;
}

function LicensedRoute({
  feature,
  children,
}: {
  feature: string;
  children: React.ReactElement;
}): React.ReactElement {
  const hasFeature = useHasFeature(feature);
  if (hasFeature) return children;
  return (
    <div className="flex flex-col items-center justify-center h-full gap-4 text-center p-8">
      <Shield size={48} className="text-muted-foreground" />
      <h2 className="text-xl font-semibold">Pro License Required</h2>
      <p className="text-sm text-muted-foreground max-w-md">
        This feature requires a Dagu Pro license. Visit the{' '}
        <Link to="/license" className="text-primary underline underline-offset-2">
          License
        </Link>{' '}
        page to activate your license.
      </p>
    </div>
  );
}

function AppInner({ config: initialConfig }: Props): React.ReactElement {
  const [config, setConfig] = React.useState(initialConfig);
  const updateConfig = React.useCallback((patch: Partial<Config>) => {
    setConfig((prev) => ({ ...prev, ...patch }));
  }, []);

  const [title, setTitle] = React.useState<string>('');
  const { preferences } = useUserPreferences();
  const theme = preferences.theme || 'dark';

  const [remoteNodes, setRemoteNodes] = React.useState<string[]>(() =>
    parseRemoteNodes(config.remoteNodes)
  );

  React.useEffect(() => {
    setRemoteNodes(parseRemoteNodes(config.remoteNodes));
  }, [config.remoteNodes]);

  const [selectedRemoteNode, setSelectedRemoteNode] = React.useState<string>(
    () => getStoredRemoteNode(remoteNodes)
  );

  const handleSelectRemoteNode = React.useCallback(
    (node: string) => {
      const validNode = remoteNodes.includes(node) ? node : 'local';
      setSelectedRemoteNode(validNode);
      localStorage.setItem(REMOTE_NODE_STORAGE_KEY, validNode);
    },
    [remoteNodes]
  );

  React.useEffect(() => {
    if (!remoteNodes.includes(selectedRemoteNode)) {
      handleSelectRemoteNode('local');
    }
  }, [remoteNodes, selectedRemoteNode, handleSelectRemoteNode]);

  React.useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.style.backgroundColor = 'var(--background)';
  }, [theme]);

  return (
    <Theme
      appearance={theme}
      accentColor="pink"
      grayColor="slate"
      radius="large"
    >
      <SWRConfig value={{ fetcher: fetchJson, onError: console.error }}>
        <AppBarContext.Provider
          value={{
            title,
            setTitle,
            remoteNodes,
            setRemoteNodes,
            selectedRemoteNode,
            selectRemoteNode: handleSelectRemoteNode,
          }}
        >
          <ConfigContext.Provider value={config}>
            <ConfigUpdateContext.Provider value={updateConfig}>
              <AuthProvider>
              <SearchStateProvider>
                <SchemaProvider>
                  <ErrorModalProvider>
                    <ToastProvider>
                      <BrowserRouter basename={config.basePath}>
                        <Routes>
                          <Route path="/login" element={<LoginPage />} />
                          <Route path="/setup" element={<SetupPage />} />
                          <Route
                            path="/*"
                            element={
                              <ProtectedRoute>
                                <AgentChatProvider>
                                  <PageContextProvider>
                                    <Layout navbarColor={config.navbarColor}>
                                      <Routes>
                                        <Route path="/" element={<Dashboard />} />
                                        <Route path="/dashboard" element={<Dashboard />} />
                                        <Route path="/dags/" element={<DAGs />} />
                                        <Route path="/dags/:fileName/:tab" element={<DAGDetails />} />
                                        <Route path="/dags/:fileName/" element={<DAGDetails />} />
                                        <Route path="/search/" element={<Search />} />
                                        <Route path="/queues" element={<Queues />} />
                                        <Route path="/dag-runs" element={<DAGRuns />} />
                                        <Route path="/dag-runs/:name/:dagRunId" element={<DAGRunDetails />} />
                                        <Route path="/system-status" element={<DeveloperElement><SystemStatus /></DeveloperElement>} />
                                        <Route path="/base-config" element={<DeveloperElement><BaseConfigPage /></DeveloperElement>} />
                                        <Route path="/users" element={<AdminElement><UsersPage /></AdminElement>} />
                                        <Route path="/remote-nodes" element={<AdminElement><RemoteNodesPage /></AdminElement>} />
                                        <Route path="/api-keys" element={<AdminElement><APIKeysPage /></AdminElement>} />
                                        <Route path="/webhooks" element={<DeveloperElement><WebhooksPage /></DeveloperElement>} />
                                        <Route path="/terminal" element={<AdminElement><TerminalPage /></AdminElement>} />
                                        <Route path="/audit-logs" element={<ManagerElement><LicensedRoute feature="audit"><AuditLogsPage /></LicensedRoute></ManagerElement>} />
                                        <Route path="/license" element={<AdminElement><LicensePage /></AdminElement>} />
                                        <Route path="/git-sync" element={<AdminElement><GitSyncPage /></AdminElement>} />
                                        <Route path="/agent-settings" element={<AdminElement><AgentSettingsPage /></AdminElement>} />
                                        <Route path="/agent-memory" element={<AdminElement><AgentMemoryPage /></AdminElement>} />
                                        <Route path="/agent-skills" element={<AdminElement><AgentSkillsPage /></AdminElement>} />
                                        <Route path="/agent-skills/new" element={<AdminElement><SkillEditorPage /></AdminElement>} />
                                        <Route path="/agent-skills/:skillId" element={<AdminElement><SkillEditorPage /></AdminElement>} />
                                        <Route path="/agent-souls" element={<AdminElement><AgentSoulsPage /></AdminElement>} />
                                        <Route path="/agent-souls/new" element={<AdminElement><SoulEditorPage /></AdminElement>} />
                                        <Route path="/agent-souls/:soulId" element={<AdminElement><SoulEditorPage /></AdminElement>} />
                                      </Routes>
                                    </Layout>
                                    {config.agentEnabled && <AgentChatModal />}
                                  </PageContextProvider>
                                </AgentChatProvider>
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
            </ConfigUpdateContext.Provider>
          </ConfigContext.Provider>
        </AppBarContext.Provider>
      </SWRConfig>
    </Theme>
  );
}

function App({ config }: Props): React.ReactElement {
  return (
    <UserPreferencesProvider>
      <AppInner config={config} />
    </UserPreferencesProvider>
  );
}

export default App;
