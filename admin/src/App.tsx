import React from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import Dashboard from "./Dashboard";
import DetailsPage from "./pages/Details";
import WorkflowsPage from "./pages/Workflows";

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
      <Dashboard {...config}>
        <Routes>
          <Route path="/" element={<WorkflowsPage />} />
          <Route path="" element={<WorkflowsPage />} />
          <Route path="/dags/" element={<WorkflowsPage />} />
          <Route path="/dags/:name" element={<DetailsPage />} />
        </Routes>
      </Dashboard>
    </BrowserRouter>
  );
}

export default App;
