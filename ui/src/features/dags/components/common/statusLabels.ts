import { StatusLabel } from '@/api/v2/schema';

/**
 * Human-readable display labels for status values.
 * Used across components that need to display status labels to users.
 */
export const STATUS_DISPLAY_LABELS: Record<StatusLabel, string> = {
  [StatusLabel.not_started]: 'Not Started',
  [StatusLabel.running]: 'Running',
  [StatusLabel.failed]: 'Failed',
  [StatusLabel.aborted]: 'Aborted',
  [StatusLabel.succeeded]: 'Succeeded',
  [StatusLabel.queued]: 'Queued',
  [StatusLabel.partially_succeeded]: 'Partial',
  [StatusLabel.waiting]: 'Waiting',
};
