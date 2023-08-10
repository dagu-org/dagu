import { DAG, DAGStatus, Node, NodeStatus, StatusFile } from './index';

export type GetDAGResponse = {
  Title: string;
  Charset: string;
  DAG?: DAGStatus;
  Graph: string;
  Definition: string;
  LogData: LogData;
  LogUrl: string;
  StepLog?: LogFile;
  ScLog?: LogFile;
  Errors: string[];
};

export type GetSearchResponse = {
  Errors: string[];
  Results: SearchResult[];
};

export type SearchResult = {
  Name: string;
  DAG?: DAG;
  Matches: Match[];
};

export type Match = {
  Line: string;
  LineNumber: number;
  StartLine: number;
};

export type LogData = {
  GridData: GridData[];
  Logs: StatusFile[];
};

export type LogFile = {
  Step?: Node;
  LogFile: string;
  Content: string;
};

export type GridData = {
  Name: string;
  Vals: NodeStatus[];
};

export type GetDAGsResponse = {
  Title: string;
  Charset: string;
  DAGs: DAGStatus[];
  Errors: string[];
  HasError: boolean;
};
