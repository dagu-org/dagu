import cronParser from 'cron-parser';
import { WorkflowListItem } from './api';

export enum SchedulerStatus {
  None = 0,
  Running,
  Error,
  Cancel,
  Success,
  Skipped_Unused,
}

export type Status = {
  RequestId: string;
  Name: string;
  Status: SchedulerStatus;
  StatusText: string;
  Pid: number;
  Nodes: Node[];
  OnExit?: Node;
  OnSuccess?: Node;
  OnFailure?: Node;
  OnCancel?: Node;
  StartedAt: string;
  FinishedAt: string;
  Log: string;
  Params: string;
};

export function Handlers(s: Status) {
  const r = [];
  if (s.OnSuccess) {
    r.push(s.OnSuccess);
  }
  if (s.OnFailure) {
    r.push(s.OnFailure);
  }
  if (s.OnCancel) {
    r.push(s.OnCancel);
  }
  if (s.OnExit) {
    r.push(s.OnExit);
  }
  return r;
}

export type Condition = {
  Condition: string;
  Expected: string;
};

export type DAG = {
  Location: string;
  Name: string;
  Schedule: Schedule[];
  Group: string;
  Tags: string[];
  Description: string;
  Env: string[];
  LogDir: string;
  HandlerOn: HandlerOn;
  Steps: Step[];
  HistRetentionDays: number;
  Preconditions: Condition[];
  MaxActiveRuns: number;
  Params: string[];
  DefaultParams?: string;
  Delay: number;
  MaxCleanUpTime: number;
};

export type Schedule = {
  Expression: string;
};

export type HandlerOn = {
  Failure: Step;
  Success: Step;
  Cancel: Step;
  Exit: Step;
};

export type DAGStatus = {
  File: string;
  Dir: string;
  DAG: DAG;
  Status?: Status;
  Suspended: boolean;
  ErrorT: string;
};

export enum DAGDataType {
  DAG = 0,
  Group,
}

export type DAGItem = DAGData | DAGGroup;

export type DAGData = {
  Type: DAGDataType.DAG;
  Name: string;
  DAGStatus: WorkflowListItem;
};

export type DAGGroup = {
  Type: DAGDataType.Group;
  Name: string;
};

export function getFirstTag(data?: DAGItem): string {
  if (!data) {
    return '';
  }
  if (data.Type == DAGDataType.DAG) {
    const tags = data.DAGStatus.DAG.Tags;
    return tags ? tags[0] : '';
  }
  return '';
}

export function getStatus(data?: DAGItem): SchedulerStatus {
  if (!data) {
    return SchedulerStatus.None;
  }
  if (data.Type == DAGDataType.DAG) {
    return data.DAGStatus.Status?.Status || SchedulerStatus.None;
  }
  return SchedulerStatus.None;
}

type KeysMatching<T extends object, V> = {
  [K in keyof T]-?: T[K] extends V ? K : never;
}[keyof T];

export function getStatusField(
  field: KeysMatching<Status, string>,
  data?: DAGItem
): string {
  if (!data) {
    return '';
  }
  if (data.Type == DAGDataType.DAG) {
    return data.DAGStatus.Status?.[field] || '';
  }
  return '';
}

export function getNextSchedule(data: WorkflowListItem): number {
  const schedules = data.DAG.Schedule;
  if (!schedules || schedules.length == 0 || data.Suspended) {
    return Number.MAX_SAFE_INTEGER;
  }
  const datesToRun = schedules.map((s) =>
    cronParser.parseExpression(s.Expression).next()
  );
  const sorted = datesToRun.sort((a, b) => a.getTime() - b.getTime());
  return sorted[0].getTime() / 1000;
}

export enum NodeStatus {
  None = 0,
  Running,
  Error,
  Cancel,
  Success,
  Skipped,
}

export type Node = {
  Step: Step;
  Log: string;
  StartedAt: string;
  FinishedAt: string;
  Status: NodeStatus;
  RetryCount: number;
  DoneCount: number;
  Error: string;
  StatusText: string;
};

export type StatusFile = {
  File: string;
  Status: Status;
};

export type Step = {
  Name: string;
  Description: string;
  Variables: string[];
  Dir: string;
  CmdWithArgs: string;
  Command: string;
  Script: string;
  Stdout: string;
  Output: string;
  Args: string[];
  Depends: string[];
  ContinueOn: ContinueOn;
  RetryPolicy?: RetryPolicy;
  RepeatPolicy: RepeatPolicy;
  MailOnError: boolean;
  Preconditions: Condition[];
};

export type RetryPolicy = {
  Limit: number;
};

export type RepeatPolicy = {
  Repeat: boolean;
  Interval: number;
};

export type ContinueOn = {
  Failure: boolean;
  Skipped: boolean;
};
