import { DAG } from "../models/Dag";
import { Node, NodeStatus } from "../models/Node";
import { StatusFile } from "../models/StatusFile";

export type GetWorkflowResponse = {
  Title: string;
  Charset: string;
  DAG?: DAG;
  Graph: string;
  Definition: string;
  LogData: LogData;
  LogUrl: string;
  Group: string;
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
