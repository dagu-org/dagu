import React from "react";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import Nav from "./components/Nav";
import DetailsPage from "./pages/Details";
import WorkflowsPage from "./pages/Workflows";

type Config = {
  title: string;
};

type Props = {
  config: Config;
};

function App({ config }: Props) {
  return (
    <div className="is-size-7">
      <Nav title={config.title} />
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<WorkflowsPage />} />
          <Route path="" element={<WorkflowsPage />} />
          <Route path="/dags/" element={<WorkflowsPage />} />
          <Route path="/dags/:name" element={<DetailsPage />} />
        </Routes>
      </BrowserRouter>
    </div>
  );
}

export default App;
