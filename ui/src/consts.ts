import { CSSProperties } from 'react';
import { NodeStatus, Status } from './api/v2/schema';

type statusColorMapping = {
  [key: number]: CSSProperties;
};
export const statusColorMapping: statusColorMapping = {
  [Status.NotStarted]: { backgroundColor: '#8a9fc4' }, // info
  [Status.Running]: { backgroundColor: '#7da87d' }, // success
  [Status.Failed]: { backgroundColor: '#c4726a', color: 'white' }, // error
  [Status.Aborted]: { backgroundColor: '#d4a574' }, // warning-muted
  [Status.Success]: { backgroundColor: '#7da87d', color: 'white' }, // success
  [Status.PartialSuccess]: { backgroundColor: '#c4956a', color: 'white' }, // warning
};

export const nodeStatusColorMapping = {
  [NodeStatus.NotStarted]: statusColorMapping[Status.NotStarted],
  [NodeStatus.Running]: statusColorMapping[Status.Running],
  [NodeStatus.Failed]: statusColorMapping[Status.Failed],
  [NodeStatus.Aborted]: statusColorMapping[Status.Aborted],
  [NodeStatus.Success]: statusColorMapping[Status.Success],
  [NodeStatus.Skipped]: { backgroundColor: '#6b635a', color: 'white' }, // muted-foreground
  [NodeStatus.PartialSuccess]: { backgroundColor: '#c4956a', color: 'white' }, // warning
};
