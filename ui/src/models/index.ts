import cronParser from 'cron-parser';
import moment from 'moment-timezone';
import { WorkflowListItem } from './api';
import { components } from '../api/v2/schema';

export type Status = {
  RequestId: string;
  Name: string;
  Status: number;
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

export function getEventHandlers(s: components['schemas']['RunDetails']) {
  const ret = [];
  if (s.onSuccess) {
    ret.push(s.onSuccess);
  }
  if (s.onFailure) {
    ret.push(s.onFailure);
  }
  if (s.onCancel) {
    ret.push(s.onCancel);
  }
  if (s.onExit) {
    ret.push(s.onExit);
  }
  return ret;
}

export type Condition = {
  Condition: string;
  Expected: string;
};

export type DAG = {
  Location?: string;
  Name: string;
  Schedule?: Schedule[];
  Group?: string;
  Tags?: string[];
  Description?: string;
  Env?: string[];
  LogDir?: string;
  HandlerOn?: HandlerOn;
  Steps?: Step[];
  HistRetentionDays?: number;
  Preconditions?: Condition[] | null;
  MaxActiveRuns?: number;
  Params?: string[];
  DefaultParams?: string;
  Delay?: number;
  MaxCleanUpTime?: number;
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
  Error: string;
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
    return tags ? tags[0] || '' : '';
  }
  return '';
}

// export function getStatus(data?: DAGItem): SchedulerStatus {
//   if (!data) {
//     return SchedulerStatus.None;
//   }
//   if (data.Type == DAGDataType.DAG) {
//     return data.DAGStatus.Status?.Status || SchedulerStatus.None;
//   }
//   return SchedulerStatus.None;
// }

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

export function getNextSchedule(
  data: components['schemas']['DAGFile']
): number {
  const schedules = data.dag.schedule;
  if (!schedules || schedules.length == 0 || data.suspended) {
    return Number.MAX_SAFE_INTEGER;
  }
  const tz = getConfig().tz || moment.tz.guess();
  const datesToRun = schedules.map((schedule) => {
    const cronTzMatch = schedule.expression.match(/(?<=CRON_TZ=)[^\s]+/);
    if (cronTzMatch) {
      const cronTz = cronTzMatch[0];
      const expressionTextWithOutCronTz = schedule.expression.replace(
        `CRON_TZ=${cronTz}`,
        ''
      );
      return cronParser
        .parseExpression(expressionTextWithOutCronTz, {
          currentDate: new Date(),
          tz: cronTz,
        })
        .next();
    }
    const expression = tz
      ? cronParser.parseExpression(schedule.expression, {
          currentDate: new Date(),
          tz,
        })
      : cronParser.parseExpression(schedule.expression);
    return expression.next();
  });
  const sorted = datesToRun.sort((a, b) => a.getTime() - b.getTime());
  if (!sorted || sorted.length == 0 || sorted[0] == null) {
    return Number.MAX_SAFE_INTEGER;
  }
  return sorted[0]?.getTime() / 1000;
}

export type Node = {
  Step: Step;
  Log: string;
  StartedAt: string;
  FinishedAt: string;
  Status: number;
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
  Description?: string;
  Variables?: string[];
  Dir?: string;
  CmdWithArgs?: string;
  Command?: string;
  Script?: string;
  Stdout?: string;
  Output?: string;
  Args?: string[];
  Depends?: string[];
  ContinueOn?: ContinueOn;
  RetryPolicy?: RetryPolicy;
  RepeatPolicy?: RepeatPolicy;
  MailOnError?: boolean;
  Preconditions?: Condition[] | null;
  Run?: string;
  Params?: string;
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
