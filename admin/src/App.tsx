import React from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import DashboardLayout from "./DashboardLayout";
import Dashboard from "./pages/Dashboard";
import View from "./pages/View";
import ViewList from "./pages/ViewList";
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
      <DashboardLayout {...config}>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="" element={<DAGs />} />
          <Route path="/views" element={<ViewList />} />
          <Route path="/views/:name" element={<View />} />
          <Route path="/dags/" element={<DAGs />} />
          <Route path="/dags/:name" element={<DAGDetails />} />
        </Routes>
      </DashboardLayout>
    </BrowserRouter>
  );
}

export default App;
