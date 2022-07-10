import React from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import Layout from "./components/layouts/Layout";
import Dashboard from "./pages/Dashboard";
import DAGDetails from "./pages/DAGDetails";
import DAGList from "./pages/DAGList";

export type Config = {
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
          <Route path="" element={<DAGList />} />
          <Route path="/dags/" element={<DAGList />} />
          <Route path="/dags/:name" element={<DAGDetails />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  );
}

export default App;
