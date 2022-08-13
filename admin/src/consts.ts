import { CSSProperties } from 'react';
import { NodeStatus } from './models';
import { SchedulerStatus } from './models';

type statusColorMapping = {
  [key: number]: CSSProperties;
};
export const statusColorMapping: statusColorMapping = {
  [SchedulerStatus.None]: { backgroundColor: '#bbbbff' },
  [SchedulerStatus.Running]: { backgroundColor: '#33ff33' },
  [SchedulerStatus.Error]: { backgroundColor: '#ee0000'},
  [SchedulerStatus.Cancel]: { backgroundColor: '#ffbbaa' },
  [SchedulerStatus.Success]: { backgroundColor: '#00bb00'},
  [SchedulerStatus.Skipped_Unused]: { backgroundColor: '#dfdfdf'},
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
