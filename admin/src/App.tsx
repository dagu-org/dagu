import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/layouts/Layout';
import Dashboard from './pages';
import DAGDetails from './pages/dags/dag';
import DAGs from './pages/dags';
import { AppBarContext } from './contexts/AppBarContext';

export type Config = {
  title: string;
  navbarColor: string;
};

type Props = {
  config: Config;
};

function App({ config }: Props) {
  const [title, setTitle] = React.useState<string>('');
  return (
    <AppBarContext.Provider
      value={{
        title,
        setTitle,
      }}
    >
      <BrowserRouter>
        <Layout {...config}>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="" element={<DAGs />} />
            <Route path="/dags/" element={<DAGs />} />
            <Route path="/dags/:name" element={<DAGDetails />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </AppBarContext.Provider>
  );
}

export default App;
