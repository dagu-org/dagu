import { CSSProperties } from "react";
import { NodeStatus } from "./models/Node";
import { SchedulerStatus } from "./models/Status";

type statusColorMapping = {
  [key: number]: CSSProperties;
};
export const statusColorMapping: statusColorMapping = {
  [SchedulerStatus.None]: { backgroundColor: "lightblue" },
  [SchedulerStatus.Running]: { backgroundColor: "lime" },
  [SchedulerStatus.Error]: { backgroundColor: "red", color: "white" },
  [SchedulerStatus.Cancel]: { backgroundColor: "pink" },
  [SchedulerStatus.Success]: { backgroundColor: "green", color: "white" },
  [SchedulerStatus.Skipped_Unused]: { backgroundColor: "gray", color: "white" },
};

export const nodeStatusColorMapping = {
  [NodeStatus.None]: statusColorMapping[SchedulerStatus.None],
  [NodeStatus.Running]: statusColorMapping[SchedulerStatus.Running],
  [NodeStatus.Error]: statusColorMapping[SchedulerStatus.Error],
  [NodeStatus.Cancel]: statusColorMapping[SchedulerStatus.Cancel],
  [NodeStatus.Success]: statusColorMapping[SchedulerStatus.Success],
  [NodeStatus.Skipped]: statusColorMapping[SchedulerStatus.Skipped_Unused],
};

export const stepTabColStyles = [
  { width: "60px" },
  { width: "200px" },
  { width: "150px" },
  { width: "150px" },
  { width: "150px" },
  { width: "130px" },
  { width: "130px" },
  { width: "100px" },
  { width: "100px" },
  {},
];
