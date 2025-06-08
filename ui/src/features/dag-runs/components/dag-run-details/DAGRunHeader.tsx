import { Calendar, Terminal, Timer } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import { DAGRunActions } from '../common';

interface DAGRunHeaderProps {
  dagRun: components['schemas']['DAGRunDetails'];
  refreshFn: () => void;
}

const DAGRunHeader: React.FC<DAGRunHeaderProps> = ({ dagRun, refreshFn }) => {
  const navigate = useNavigate();

  // Format duration utility function
  const formatDuration = (startDate: string, endDate: string): string => {
    if (!startDate || !endDate) return '--';

    const duration = dayjs.duration(dayjs(endDate).diff(dayjs(startDate)));
    const hours = Math.floor(duration.asHours());
    const minutes = duration.minutes();
    const seconds = duration.seconds();

    if (hours > 0) return `${hours}h ${minutes}m ${seconds}s`;
    if (minutes > 0) return `${minutes}m ${seconds}s`;
    return `${seconds}s`;
  };

  const handleRootDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(`/dag-runs/${dagRun.rootDAGRunName}/${dagRun.rootDAGRunId}`);
  };

  const handleParentDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    if (dagRun.parentDAGRunId) {
      const searchParams = new URLSearchParams();
      searchParams.set('childDAGRunId', dagRun.parentDAGRunId);
      searchParams.set('dagRunId', dagRun.rootDAGRunId);
      searchParams.set('dagRunName', dagRun.rootDAGRunName);
      navigate(
        `/dag-runs/${dagRun.rootDAGRunName}/${dagRun.rootDAGRunId}?${searchParams.toString()}`
      );
    }
  };

  return (
    <div className="bg-gradient-to-br from-slate-50 via-white to-slate-50 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 rounded-2xl p-6 mb-6 border border-slate-200 dark:border-slate-700 shadow-sm">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6 mb-4">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-slate-600 dark:text-slate-400 mb-2">
            {dagRun.rootDAGRunId !== dagRun.dagRunId && (
              <>
                <a
                  href={`/dag-runs/${dagRun.rootDAGRunName}/${dagRun.rootDAGRunId}`}
                  onClick={handleRootDAGRunClick}
                  className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                >
                  {dagRun.rootDAGRunName}
                </a>
                <span className="text-slate-400 dark:text-slate-500 mx-1">
                  /
                </span>
              </>
            )}

            {dagRun.parentDAGRunName &&
              dagRun.parentDAGRunId &&
              dagRun.parentDAGRunName !== dagRun.rootDAGRunName &&
              dagRun.parentDAGRunName !== dagRun.name && (
                <>
                  <a
                    href="#"
                    onClick={handleParentDAGRunClick}
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                  >
                    {dagRun.parentDAGRunName}
                  </a>
                  <span className="text-slate-400 dark:text-slate-500 mx-1">
                    /
                  </span>
                </>
              )}
          </nav>

          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 truncate">
            {dagRun.name}
          </h1>
        </div>
      </div>

      {/* Status and metadata row */}
      {dagRun.status != Status.NotStarted && (
        <div className="flex flex-wrap items-center gap-2 lg:gap-6">
          {/* Status and actions */}
          <div className="flex items-center gap-3">
            {dagRun.status && (
              <StatusChip status={dagRun.status} size="md">
                {dagRun.statusLabel || ''}
              </StatusChip>
            )}
            <DAGRunActions
              dagRun={dagRun}
              name={dagRun.name}
              refresh={refreshFn}
              displayMode="compact"
              isRootLevel={dagRun.rootDAGRunId === dagRun.dagRunId}
            />
          </div>

          {/* Metadata items */}
          <div className="flex flex-wrap items-center gap-4 lg:gap-6 text-sm">
            <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200 bg-slate-200 dark:bg-slate-700 rounded-md px-3 py-1.5 border">
              <Calendar className="h-4 w-4 text-slate-600 dark:text-slate-400" />
              <span className="font-medium text-xs">
                {dagRun?.startedAt
                  ? `${dayjs(dagRun.startedAt).format('MMM D, HH:mm:ss')} ${dayjs(dagRun.startedAt).format('z')}`
                  : '--'}
              </span>
            </div>

            <div className="flex items-center gap-2 text-slate-800 dark:text-slate-200 bg-slate-200 dark:bg-slate-700 rounded-md px-3 py-1.5 border">
              <Timer className="h-4 w-4 text-slate-600 dark:text-slate-400" />
              <span className="font-medium text-xs">
                {dagRun.finishedAt
                  ? formatDuration(dagRun.startedAt, dagRun.finishedAt)
                  : dagRun.startedAt
                    ? formatDuration(dagRun.startedAt, dayjs().toISOString())
                    : '--'}
              </span>
            </div>

            <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 ml-auto">
              <span className="font-medium text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                Run ID
              </span>
              <code className="bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 px-3 py-1.5 rounded-md text-xs font-mono border">
                {dagRun.dagRunId}
              </code>
            </div>
          </div>
        </div>
      )}

      {/* Parameters - Show if present */}
      {dagRun.params && (
        <div className="mt-4 border-t border-slate-200 dark:border-slate-700 pt-4">
          <div className="flex items-center gap-2 mb-2">
            <Terminal className="h-4 w-4 text-slate-500" />
            <span className="text-sm font-semibold text-slate-700 dark:text-slate-300">
              Parameters
            </span>
          </div>
          <div className="bg-slate-200 dark:bg-slate-700 rounded-md px-4 py-3 font-mono text-sm text-slate-800 dark:text-slate-200 max-h-[120px] overflow-y-auto border">
            {dagRun.params}
          </div>
        </div>
      )}
    </div>
  );
};

export default DAGRunHeader;
