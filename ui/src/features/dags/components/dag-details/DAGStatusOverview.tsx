/**
 * DAGStatusOverview component displays summary information about a DAG run.
 *
 * @module features/dags/components/dag-details
 */
import { FileText } from 'lucide-react';
import { Link } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import LabeledItem from '../../../../ui/LabeledItem';
import StatusChip from '../../../../ui/StatusChip';

/**
 * Props for the DAGStatusOverview component
 */
type Props = {
  /** DAG run details */
  status?: components['schemas']['RunDetails'];
  /** DAG file ID */
  fileId: string;
  /** Request ID of the execution */
  requestId?: string;
};

/**
 * DAGStatusOverview displays summary information about a DAG run
 * including status, request ID, timestamps, and parameters
 */
function DAGStatusOverview({ status, fileId, requestId = '' }: Props) {
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  if (requestId) {
    searchParams.set('requestId', requestId);
  }
  const url = `/dags/${fileId}/scheduler-log?${searchParams.toString()}`;

  // Don't render if no status is provided
  if (!status) {
    return null;
  }

  // Format timestamps for better readability if they exist
  const formatTimestamp = (timestamp: string | undefined) => {
    if (!timestamp || timestamp == '-') return '-';
    try {
      const date = new Date(timestamp);
      return date.toLocaleString();
    } catch (e) {
      return timestamp;
    }
  };

  return (
    <div>
      <div className="mb-3">
        <div className="flex items-center justify-between">
          <StatusChip status={status.status} size="md">
            {status.statusText}
          </StatusChip>
        </div>
      </div>

      <div className="h-px w-full bg-slate-200 dark:bg-slate-700 my-3"></div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        {status.requestId && (
          <div className="space-y-2">
            <LabeledItem label="Request ID">
              <div className="flex flex-row gap-2">
                <div className="p-1.5 bg-slate-100 dark:bg-slate-800 rounded-md font-medium text-xs text-slate-700 dark:text-slate-300">
                  {status.requestId}
                </div>
                <Link
                  to={url}
                  className="inline-flex items-center gap-2 text-slate-600 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200 transition-colors duration-200 cursor-pointer"
                >
                  <FileText className="h-4 w-4" />
                </Link>
              </div>
            </LabeledItem>
          </div>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 mt-3">
        <LabeledItem label="Started At">
          <span className="font-medium text-slate-700 dark:text-slate-300 text-sm">
            {formatTimestamp(status.startedAt)}
          </span>
        </LabeledItem>

        <LabeledItem label="Finished At">
          <span className="font-medium text-slate-700 dark:text-slate-300 text-sm">
            {formatTimestamp(status.finishedAt)}
          </span>
        </LabeledItem>
      </div>

      {status.params && (
        <div className="mt-3">
          <LabeledItem label="Parameters" className="items-start">
            <div className="p-2 bg-slate-100 dark:bg-slate-800 rounded-md font-medium text-xs text-slate-700 dark:text-slate-300 font-mono max-h-[120px] overflow-y-auto w-full">
              {status.params}
            </div>
          </LabeledItem>
        </div>
      )}
    </div>
  );
}

export default DAGStatusOverview;
