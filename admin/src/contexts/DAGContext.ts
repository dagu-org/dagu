import React from "react";
import { GetDAGResponse } from "../api/DAG";
import { DetailTabId } from "../models/DAG";

export const DAGContext = React.createContext({
  refresh: () => {},
  data: null as GetDAGResponse | null,
  name: "",
  tab: DetailTabId.Status,
  group: "",
});
