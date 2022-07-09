import React from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import Layout from "./components/layouts/Layout";
import Dashboard from "./pages/Dashboard";
import DAGDetails from "./pages/DAGDetails";
import DAGs from "./pages/DAGs";

type Config = {
  title: string;
  navbarColor: string;
};

type Props = {
  config: Config;
};

function App({ config }: Props) {
  return (
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
  );
}

export default App;
