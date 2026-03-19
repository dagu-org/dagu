import { NodeStatus } from '@/api/v1/schema';
import { describe, expect, it } from 'vitest';
import {
  getNodeStatusIcon,
  getStatusClass,
  getStatusColors,
  isActiveNodeStatus,
} from '../status-utils';

describe('status-utils', () => {
  it('treats retrying as an active warning status', () => {
    expect(isActiveNodeStatus(NodeStatus.Retrying)).toBe(true);
    expect(getStatusClass(NodeStatus.Retrying)).toBe('status-warning');
    expect(getStatusColors(NodeStatus.Retrying)).toMatchObject({
      bgClass: 'bg-[#e37400] dark:bg-[#fdd663]',
      animation: 'animate-pulse',
    });
  });

  it('exposes a distinct retrying icon', () => {
    expect(getNodeStatusIcon(NodeStatus.Retrying)).toBe('↻');
  });
});
