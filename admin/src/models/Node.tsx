import { Step } from './Step';

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
