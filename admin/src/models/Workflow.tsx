import { DAG } from "./Dag";
import { Group } from "./Group";

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