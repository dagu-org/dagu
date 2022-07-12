import { Config } from './Config';
import { SchedulerStatus, Status } from './Status';
import cronParser from 'cron-parser';

export type DAG = {
  File: string;
  Dir: string;
  Config: Config;
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
  DAG: DAG;
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
    const tags = data.DAG.Config.Tags;
    return tags ? tags[0] : '';
  }
  return '';
}

export function getStatus(data?: DAGItem): SchedulerStatus {
  if (!data) {
    return SchedulerStatus.None;
  }
  if (data.Type == DAGDataType.DAG) {
    return data.DAG.Status?.Status || SchedulerStatus.None;
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
    return data.DAG.Status?.[field] || '';
  }
  return '';
}

export function getNextSchedule(data: DAG): number {
  const schedules = data.Config.ScheduleExp;
  if (!schedules || schedules.length == 0) {
    return Number.MAX_SAFE_INTEGER;
  }
  const datesToRun = schedules.map((s) => cronParser.parseExpression(s).next());
  const sorted = datesToRun.sort((a, b) => a.getTime() - b.getTime());
  return sorted[0].getTime();
}

export enum DetailTabId {
  Status = '0',
  Config = '1',
  History = '2',
  StepLog = '3',
  ScLog = '4',
  None = '5',
}
