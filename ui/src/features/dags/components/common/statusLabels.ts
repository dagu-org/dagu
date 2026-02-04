import { StatusLabel } from '@/api/v1/schema';

/**
 * Human-readable display labels for status values.
 * Used across components that need to display status labels to users.
 * Uses lowercase for minimal, text-only status display.
 */
export const STATUS_DISPLAY_LABELS: Record<StatusLabel, string> = {
  [StatusLabel.not_started]: 'not started',
  [StatusLabel.running]: 'running',
  [StatusLabel.failed]: 'failed',
  [StatusLabel.aborted]: 'aborted',
  [StatusLabel.succeeded]: 'success',
  [StatusLabel.queued]: 'queued',
  [StatusLabel.partially_succeeded]: 'partial',
  [StatusLabel.waiting]: 'waiting',
  [StatusLabel.rejected]: 'rejected',
};
