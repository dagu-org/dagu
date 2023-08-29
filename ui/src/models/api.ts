import { DAG, DAGStatus, Node, NodeStatus, SchedulerStatus, Status, StatusFile } from './index';

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

export type ListWorkflowsResponse = {
  Title: string;
  Charset: string;
  DAGs: WorkflowListItem[];
  Errors: string[];
  HasError: boolean;
};

export type WorkflowListItem = {
  File: string;
  Dir: string;
  Status?: WorkflowStatus;
  Suspended: boolean;
  ErrorT: string;
  DAG: WorkflowDefinition;
};

export type WorkflowDefinition = {
  Name: string;
  Group: string;
  Tags: string[];
  Description: string;
  Params: string[];
  DefaultParams?: string;
};

export type WorkflowStatus = {
  RequestId: string;
  Name: string;
  Status: SchedulerStatus;
  StatusText: string;
  Pid: number;
  StartedAt: string;
  FinishedAt: string;
  Log: string;
  Params: string;
};