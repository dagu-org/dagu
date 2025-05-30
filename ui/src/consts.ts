import { CSSProperties } from 'react';
import { NodeStatus, Status } from './api/v2/schema';

type statusColorMapping = {
  [key: number]: CSSProperties;
};
export const statusColorMapping: statusColorMapping = {
  [Status.NotStarted]: { backgroundColor: 'lightblue' },
  [Status.Running]: { backgroundColor: 'lime' },
  [Status.Failed]: { backgroundColor: 'red', color: 'white' },
  [Status.Cancelled]: { backgroundColor: 'pink' },
  [Status.Success]: { backgroundColor: 'green', color: 'white' },
};

export const nodeStatusColorMapping = {
  [NodeStatus.NotStarted]: statusColorMapping[Status.NotStarted],
  [NodeStatus.Running]: statusColorMapping[Status.Running],
  [NodeStatus.Failed]: statusColorMapping[Status.Failed],
  [NodeStatus.Cancelled]: statusColorMapping[Status.Cancelled],
  [NodeStatus.Success]: statusColorMapping[Status.Success],
  [NodeStatus.Skipped]: { backgroundColor: 'gray', color: 'white' },
};
