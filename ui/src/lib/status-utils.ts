import { NodeStatus, Status } from '@/api/v1/schema';

/**
 * Get Tailwind CSS utility class for Status or NodeStatus enum.
 * Uses utility classes defined in global.css.
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
    case NodeStatus.Skipped:
      return 'status-neutral';

    case Status.PartialSuccess:
    case Status.Waiting:
    case NodeStatus.PartialSuccess:
    case NodeStatus.Waiting:
      return 'status-warning';

    case Status.Aborted:
    case NodeStatus.Aborted:
      return 'status-aborted';

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
  const isWaiting = status === Status.Waiting || status === NodeStatus.Waiting;

  switch (baseClass) {
    case 'status-success':
      return {
        bgClass: 'bg-[#1e8e3e] dark:bg-[#81c995]',
        textClass: 'text-[#1e8e3e] dark:text-[#81c995]',
        borderClass: 'border-[#1e8e3e] dark:border-[#81c995]',
        animation: '',
      };

    case 'status-failed':
      return {
        bgClass: 'bg-[#d93025] dark:bg-[#f28b82]',
        textClass: 'text-[#d93025] dark:text-[#f28b82]',
        borderClass: 'border-[#d93025] dark:border-[#f28b82]',
        animation: '',
      };

    case 'status-running':
      return {
        bgClass: 'bg-[#34a853] dark:bg-[#81c995]',
        textClass: 'text-[#34a853] dark:text-[#81c995]',
        borderClass: 'border-[#34a853] dark:border-[#81c995]',
        animation: '',
      };

    case 'status-neutral':
      return {
        bgClass: 'bg-[#5f6368] dark:bg-[#9aa0a6]',
        textClass: 'text-[#5f6368] dark:text-[#9aa0a6]',
        borderClass: 'border-[#5f6368] dark:border-[#9aa0a6]',
        animation: '',
      };

    case 'status-warning':
      return {
        bgClass: 'bg-[#e37400] dark:bg-[#fdd663]',
        textClass: 'text-[#e37400] dark:text-[#fdd663]',
        borderClass: 'border-[#e37400] dark:border-[#fdd663]',
        animation: isWaiting ? 'animate-pulse' : '',
      };

    case 'status-aborted':
      return {
        bgClass: 'bg-[#d946ef] dark:bg-[#e879f9]',
        textClass: 'text-[#d946ef] dark:text-[#e879f9]',
        borderClass: 'border-[#d946ef] dark:border-[#e879f9]',
        animation: '',
      };

    default:
      return {
        bgClass: 'bg-[#5f6368] dark:bg-[#9aa0a6]',
        textClass: 'text-[#5f6368] dark:text-[#9aa0a6]',
        borderClass: 'border-[#5f6368] dark:border-[#9aa0a6]',
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
