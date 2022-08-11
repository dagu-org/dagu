import { DAGStatus, Node, NodeStatus, StatusFile } from '../models';

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
  GridData: DagStatus[];
  Logs: StatusFile[];
};

export type LogFile = {
  Step?: Node;
  LogFile: string;
  Content: string;
};

export type DagStatus = {
  Name: string;
  Vals: NodeStatus[];
};
