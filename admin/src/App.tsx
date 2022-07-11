import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import Layout from './components/layouts/Layout';
import Dashboard from './pages/Dashboard';
import DAGDetails from './pages/DAGDetails';
import DAGList from './pages/DAGList';
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
            <Route path="" element={<DAGList />} />
            <Route path="/dags/" element={<DAGList />} />
            <Route path="/dags/:name" element={<DAGDetails />} />
          </Routes>
        </Layout>
      </BrowserRouter>
    </AppBarContext.Provider>
  );
}

export default App;
