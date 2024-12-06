import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './Layout';
import Dashboard from './pages';
import DAGDetails from './pages/dags/dag';
import DAGs from './pages/dags';
import { AppBarContext } from './contexts/AppBarContext';
import { SWRConfig } from 'swr';
import fetchJson from './lib/fetchJson';
import Search from './pages/search';
import { UserPreferencesProvider } from './contexts/UserPreference';
import { Config, ConfigContext } from './contexts/ConfigContext';
import moment from 'moment-timezone';

type Props = {
  config: Config;
};

function App({ config }: Props) {
  const [title, setTitle] = React.useState<string>('');
  config.tz ||= moment.tz.guess();
  const remoteNodes = config.remoteNodes
    .split(',')
    .filter(Boolean)
    .map((node) => node.trim());
  if (!remoteNodes.includes('local')) {
    remoteNodes.unshift('local');
  }
  const [selectedRemoteNode, setSelectedRemoteNode] =
    React.useState<string>('local');

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
          selectRemoteNode: setSelectedRemoteNode,
        }}
      >
        <ConfigContext.Provider value={config}>
          <UserPreferencesProvider>
            <BrowserRouter basename={config.basePath}>
              <Layout {...config}>
                <Routes>
                  <Route path="/" element={<Dashboard />} />
                  <Route path="/dashboard" element={<Dashboard />} />
                  <Route path="/dags/" element={<DAGs />} />
                  <Route path="/dags/:name/:tab" element={<DAGDetails />} />
                  <Route path="/dags/:name/" element={<DAGDetails />} />
                  <Route path="/search/" element={<Search />} />
                </Routes>
              </Layout>
            </BrowserRouter>
          </UserPreferencesProvider>
        </ConfigContext.Provider>
      </AppBarContext.Provider>
    </SWRConfig>
  );
}

export default App;
