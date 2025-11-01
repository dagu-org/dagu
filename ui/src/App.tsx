import React from 'react';
import { BrowserRouter, Route, Routes } from 'react-router-dom';
import { SWRConfig } from 'swr';
import { ToastProvider } from './components/ui/simple-toast';
import { AppBarContext } from './contexts/AppBarContext';
import { Config, ConfigContext } from './contexts/ConfigContext';
import { SearchStateProvider } from './contexts/SearchStateContext';
import { UserPreferencesProvider } from './contexts/UserPreference';
import Layout from './layouts/Layout';
import fetchJson from './lib/fetchJson';
import Dashboard from './pages';
import DAGs from './pages/dags';
import DAGDetails from './pages/dags/dag';
import Search from './pages/search';
import DAGRuns from './pages/dag-runs';
import DAGRunDetails from './pages/dag-runs/dag-run';
import Queues from './pages/queues';
import Workers from './pages/workers';
import SystemStatus from './pages/system-status';

type Props = {
  config: Config;
};

function App({ config }: Props) {
  const [title, setTitle] = React.useState<string>('');

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
    // Ensure the stored node is actually in the available nodes list
    if (storedNode && remoteNodes.includes(storedNode)) {
      return storedNode;
    }
    return 'local'; // Default
  };

  const [selectedRemoteNode, setSelectedRemoteNode] =
    React.useState<string>(getInitialNode);

  // Function to update state and localStorage
  const handleSelectRemoteNode = (node: string) => {
    if (remoteNodes.includes(node)) {
      setSelectedRemoteNode(node);
      localStorage.setItem(localStorageKey, node);
    } else {
      console.warn(`Attempted to select invalid remote node: ${node}`);
      // Optionally reset to default or handle error
      setSelectedRemoteNode('local');
      localStorage.setItem(localStorageKey, 'local');
    }
  };

  // Effect to update state if remoteNodes list changes and current selection is invalid
  React.useEffect(() => {
    if (!remoteNodes.includes(selectedRemoteNode)) {
      handleSelectRemoteNode('local'); // Reset to default if current selection is no longer valid
    }
    // We only want this effect to run if remoteNodes changes,
    // or selectedRemoteNode becomes invalid relative to remoteNodes.
    // Adding handleSelectRemoteNode to deps would cause unnecessary runs.
  }, [remoteNodes, selectedRemoteNode]);

  return (
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
          <UserPreferencesProvider>
            <SearchStateProvider>
              <ToastProvider>
                <BrowserRouter basename={config.basePath}>
                  <Layout {...config}>
                    <Routes>
                      <Route path="/" element={<Dashboard />} />
                      <Route path="/dashboard" element={<Dashboard />} />
                      <Route path="/system-status" element={<SystemStatus />} />
                      <Route path="/dags/" element={<DAGs />} />
                      <Route
                        path="/dags/:fileName/:tab"
                        element={<DAGDetails />}
                      />
                      <Route path="/dags/:fileName/" element={<DAGDetails />} />
                      <Route path="/search/" element={<Search />} />
                      <Route path="/queues" element={<Queues />} />
                      <Route path="/dag-runs" element={<DAGRuns />} />
                      <Route
                        path="/dag-runs/:name/:dagRunId"
                        element={<DAGRunDetails />}
                      />
                      <Route path="/workers" element={<Workers />} />
                    </Routes>
                  </Layout>
                </BrowserRouter>
              </ToastProvider>
            </SearchStateProvider>
          </UserPreferencesProvider>
        </ConfigContext.Provider>
      </AppBarContext.Provider>
    </SWRConfig>
  );
}

export default App;
