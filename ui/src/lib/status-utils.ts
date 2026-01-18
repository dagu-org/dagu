import { NodeStatus, Status } from '@/api/v2/schema';

/**
 * Get Tailwind CSS utility class for Status or NodeStatus enum.
 * Uses utility classes defined in global.css (lines 280-297).
 *
 * @param status - The Status or NodeStatus enum value
 * @returns The corresponding CSS utility class name
 */
export function getStatusClass(status?: Status | NodeStatus): string {
  switch (status) {
    case Status.Success:
    case NodeStatus.Success:
      return 'status-success';

    case Status.Failed:
    case Status.Rejected:
    case NodeStatus.Failed:
    case NodeStatus.Rejected:
      return 'status-failed';

    case Status.Running:
    case NodeStatus.Running:
      return 'status-running';

    case Status.Queued:
    case Status.NotStarted:
    case NodeStatus.NotStarted:
      return 'status-info';

    case NodeStatus.Skipped:
      return 'status-info';

    case Status.PartialSuccess:
    case Status.Waiting:
    case Status.Aborted:
    case NodeStatus.PartialSuccess:
    case NodeStatus.Waiting:
    case NodeStatus.Aborted:
      return 'status-warning';

    default:
      return 'status-muted';
  }
}

/**
 * Get separate background, text, border, and animation classes for status.
 * Useful for components that need granular control over styling.
 *
 * @param status - The Status or NodeStatus enum value
 * @returns Object with bgClass, textClass, borderClass, and animation
 */
export function getStatusColors(
  status?: Status | NodeStatus
): {
  bgClass: string;
  textClass: string;
  borderClass: string;
  animation: string;
} {
  const baseClass = getStatusClass(status);
  const isRunning = status === Status.Running || status === NodeStatus.Running;
  const isWaiting = status === Status.Waiting || status === NodeStatus.Waiting;

  switch (baseClass) {
    case 'status-success':
      return {
        bgClass: 'bg-success',
        textClass: 'text-success',
        borderClass: 'border-success',
        animation: '',
      };

    case 'status-failed':
      return {
        bgClass: 'bg-destructive',
        textClass: 'text-destructive',
        borderClass: 'border-destructive',
        animation: '',
      };

    case 'status-running':
      return {
        bgClass: 'bg-primary',
        textClass: 'text-primary',
        borderClass: 'border-primary',
        animation: 'animate-pulse',
      };

    case 'status-info':
      return {
        bgClass: 'bg-info',
        textClass: 'text-info',
        borderClass: 'border-info',
        animation: '',
      };

    case 'status-warning':
      return {
        bgClass: 'bg-warning',
        textClass: 'text-warning',
        borderClass: 'border-warning',
        animation: isWaiting ? 'animate-pulse' : '',
      };

    default:
      return {
        bgClass: 'bg-muted',
        textClass: 'text-muted-foreground',
        borderClass: 'border-muted-foreground',
        animation: '',
      };
  }
}

/**
 * Get the display icon for a NodeStatus value.
 *
 * @param status - The NodeStatus enum value
 * @returns Unicode character representing the status
 */
export function getNodeStatusIcon(status: NodeStatus): string {
  switch (status) {
    case NodeStatus.Success:
      return '✓';
    case NodeStatus.Failed:
      return '✕';
    case NodeStatus.Rejected:
      return '⊘';
    case NodeStatus.NotStarted:
      return '○';
    case NodeStatus.Skipped:
      return '―';
    case NodeStatus.PartialSuccess:
      return '◐';
    case NodeStatus.Waiting:
      return '□';
    case NodeStatus.Aborted:
      return '■';
    case NodeStatus.Running:
      return ''; // Running uses BrailleSpinner
    default:
      return '○';
  }
}
