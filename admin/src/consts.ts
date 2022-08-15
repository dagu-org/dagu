import { CSSProperties } from 'react';
import { NodeStatus } from './models';
import { SchedulerStatus } from './models';

type statusColorMapping = {
  [key: number]: CSSProperties;
};
export const statusColorMapping: statusColorMapping = {
  [SchedulerStatus.None]: { backgroundColor: 'lightblue' },
  [SchedulerStatus.Running]: { backgroundColor: 'lime' },
  [SchedulerStatus.Error]: { backgroundColor: 'red', color: 'white' },
  [SchedulerStatus.Cancel]: { backgroundColor: 'pink' },
  [SchedulerStatus.Success]: { backgroundColor: 'green', color: 'white' },
  [SchedulerStatus.Skipped_Unused]: { backgroundColor: 'gray', color: 'white' },
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
  { maxWidth: '60px' },
  { maxWidth: '200px' },
  { maxWidth: '150px' },
  { maxWidth: '150px' },
  { maxWidth: '150px' },
  { maxWidth: '130px' },
  { maxWidth: '130px' },
  { maxWidth: '100px' },
  { maxWidth: '100px' },
  {},
];
