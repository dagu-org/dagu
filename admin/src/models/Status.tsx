import { Node } from './Node';

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
