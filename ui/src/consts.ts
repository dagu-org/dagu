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
  [Status.NotStarted]: {
    backgroundColor: 'var(--muted)',
    color: 'var(--muted-foreground)',
  },
  [Status.Running]: {
    backgroundColor: 'var(--success)',
    color: 'var(--primary-foreground)',
  },
  [Status.Failed]: {
    backgroundColor: 'var(--destructive)',
    color: 'var(--destructive-foreground)',
  },
  [Status.Aborted]: {
    backgroundColor: 'var(--warning)',
    color: 'var(--primary-foreground)',
  },
  [Status.Success]: {
    backgroundColor: 'var(--success)',
    color: 'var(--primary-foreground)',
  },
  [Status.PartialSuccess]: {
    backgroundColor: 'var(--warning)',
    color: 'var(--primary-foreground)',
  },
  [Status.Rejected]: {
    backgroundColor: 'var(--destructive)',
    color: 'var(--destructive-foreground)',
  },
};

export const nodeStatusColorMapping = {
  [NodeStatus.NotStarted]: statusColorMapping[Status.NotStarted],
  [NodeStatus.Running]: statusColorMapping[Status.Running],
  [NodeStatus.Failed]: statusColorMapping[Status.Failed],
  [NodeStatus.Aborted]: statusColorMapping[Status.Aborted],
  [NodeStatus.Success]: statusColorMapping[Status.Success],
  [NodeStatus.Skipped]: {
    backgroundColor: 'var(--muted)',
    color: 'var(--muted-foreground)',
  },
  [NodeStatus.PartialSuccess]: statusColorMapping[Status.PartialSuccess],
  [NodeStatus.Rejected]: statusColorMapping[Status.Rejected],
};
