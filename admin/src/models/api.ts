import { DAGStatus, Node, NodeStatus, StatusFile } from './index';

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
