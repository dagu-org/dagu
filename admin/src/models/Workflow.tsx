import { DAG } from "./Dag";
import { Group } from "./Group";
import { SchedulerStatus, Status } from "./Status";

export enum WorkflowDataType {
  Workflow = 0,
  Group,
}

export type WorkflowData = Workflow | WorkflowGroup;

export type Workflow = {
  Type: WorkflowDataType.Workflow;
  Name: string;
  DAG: DAG;
};

export type WorkflowGroup = {
  Type: WorkflowDataType.Group;
  Name: string;
  Group: Group;
};

export function getFirstTag(data?: WorkflowData): string {
  if (!data) {
    return "";
  }
  if (data.Type == WorkflowDataType.Workflow) {
    const tags = data.DAG.Config.Tags;
    return tags ? tags[0] : "";
  }
  return "";
}

export function getStatus(data?: WorkflowData): SchedulerStatus {
  if (!data) {
    return SchedulerStatus.None;
  }
  if (data.Type == WorkflowDataType.Workflow) {
    return data.DAG.Status?.Status || SchedulerStatus.None;
  }
  return SchedulerStatus.None;
}

type KeysMatching<T extends object, V> = {
  [K in keyof T]-?: T[K] extends V ? K : never
}[keyof T];

export function getStatusField(
  field: KeysMatching<Status, string>,
  data?: WorkflowData,
): string {
  if (!data) {
    return "";
  }
  if (data.Type == WorkflowDataType.Workflow) {
    return data.DAG.Status?.[field] || "";
  }
  return "";
}
