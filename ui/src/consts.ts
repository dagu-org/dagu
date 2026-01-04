import { CSSProperties } from 'react';
import { NodeStatus, Status } from './api/v2/schema';

/**
 * Status color mappings - Professional gray theme
 * Using Tailwind color palette for consistency
 */

type statusColorMapping = {
  [key: number]: CSSProperties;
};

export const statusColorMapping: statusColorMapping = {
  [Status.NotStarted]: { backgroundColor: '#6b7280', color: 'white' }, // gray-500
  [Status.Running]: { backgroundColor: '#16a34a', color: 'white' }, // green-600
  [Status.Failed]: { backgroundColor: '#dc2626', color: 'white' }, // red-600
  [Status.Aborted]: { backgroundColor: '#d97706', color: 'white' }, // amber-600
  [Status.Success]: { backgroundColor: '#16a34a', color: 'white' }, // green-600
  [Status.PartialSuccess]: { backgroundColor: '#ca8a04', color: 'white' }, // yellow-600
  [Status.Rejected]: { backgroundColor: '#b91c1c', color: 'white' }, // red-700
};

export const nodeStatusColorMapping = {
  [NodeStatus.NotStarted]: statusColorMapping[Status.NotStarted],
  [NodeStatus.Running]: statusColorMapping[Status.Running],
  [NodeStatus.Failed]: statusColorMapping[Status.Failed],
  [NodeStatus.Aborted]: statusColorMapping[Status.Aborted],
  [NodeStatus.Success]: statusColorMapping[Status.Success],
  [NodeStatus.Skipped]: { backgroundColor: '#9ca3af', color: 'white' }, // gray-400
  [NodeStatus.PartialSuccess]: statusColorMapping[Status.PartialSuccess],
  [NodeStatus.Rejected]: statusColorMapping[Status.Rejected],
};
