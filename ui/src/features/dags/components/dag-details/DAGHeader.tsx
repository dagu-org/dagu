import { Calendar, Terminal, Timer } from 'lucide-react';
import React from 'react';
import { useNavigate } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import { RootDAGRunContext } from '../../contexts/RootDAGRunContext';
import { DAGActions } from '../common';

interface DAGHeaderProps {
  dag: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  currentDAGRun: components['schemas']['DAGRunDetails'];
  fileName: string;
  refreshFn: () => void;
  formatDuration: (startDate: string, endDate: string) => string;
  navigateToStatusTab?: () => void;
}

const DAGHeader: React.FC<DAGHeaderProps> = ({
  dag,
  currentDAGRun,
  fileName,
  refreshFn,
  formatDuration,
  navigateToStatusTab,
}) => {
  const navigate = useNavigate();
  const rootDAGRunContext = React.useContext(RootDAGRunContext);

  // Use the DAG-run from context if available, otherwise use the prop
  const dagRunToDisplay = rootDAGRunContext.data || currentDAGRun;

  const handleRootDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`
    );
  };

  const handleParentDAGRunClick = (e: React.MouseEvent) => {
    e.preventDefault();
    navigate(
      `/dags/${fileName}?childDAGRunId=${dagRunToDisplay.parentDAGRunId}&dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`
    );
  };

  return (
    <div className="bg-gradient-to-br from-slate-50 via-white to-slate-50 dark:from-slate-900 dark:via-slate-800 dark:to-slate-900 rounded-2xl p-6 mb-6 border border-slate-200 dark:border-slate-700 shadow-sm">
      {/* Header with title and actions */}
      <div className="flex items-start justify-between gap-6 mb-4">
        <div className="flex-1 min-w-0">
          {/* Breadcrumb navigation */}
          <nav className="flex flex-wrap items-center gap-1.5 text-sm text-slate-600 dark:text-slate-400 mb-2">
            {dagRunToDisplay.rootDAGRunId !== dagRunToDisplay.dagRunId && (
              <>
                <a
                  href={`/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`}
                  onClick={handleRootDAGRunClick}
                  className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                >
                  {dagRunToDisplay.rootDAGRunName}
                </a>
                <span className="text-slate-400 dark:text-slate-500 mx-1">
                  /
                </span>
              </>
            )}

            {dagRunToDisplay.parentDAGRunName &&
              dagRunToDisplay.parentDAGRunId &&
              dagRunToDisplay.parentDAGRunName !==
                dagRunToDisplay.rootDAGRunName &&
              dagRunToDisplay.parentDAGRunName !== dagRunToDisplay.name && (
                <>
                  <a
                    href={`/dags/${fileName}?dagRunId=${dagRunToDisplay.rootDAGRunId}&childDAGRunId=${dagRunToDisplay.parentDAGRunId}&dagRunName=${encodeURIComponent(dagRunToDisplay.rootDAGRunName)}`}
                    onClick={handleParentDAGRunClick}
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300 hover:underline transition-colors font-medium"
                  >
                    {dagRunToDisplay.parentDAGRunName}
                  </a>
                  <span className="text-slate-400 dark:text-slate-500 mx-1">
                    /
                  </span>
                </>
              )}
          </nav>

          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100 truncate">
            {dagRunToDisplay.name}
          </h1>
        </div>

        {/* Actions */}
        {dagRunToDisplay.dagRunId === dagRunToDisplay.rootDAGRunId && (
          <div className="flex-shrink-0">
            <DAGActions
              status={dagRunToDisplay}
              dag={dag}
              fileName={fileName}
              refresh={refreshFn}
              displayMode="compact"
              navigateToStatusTab={navigateToStatusTab}
            />
          </div>
        )}
      </div>

      {/* Status and metadata row */}
      {dagRunToDisplay.status !== undefined &&
        dagRunToDisplay.status !== null && (
          <div className="flex flex-wrap items-center gap-4 text-sm">
            {/* Status */}
            {dagRunToDisplay.status && (
              <div className="flex items-center gap-2">
                <StatusChip status={dagRunToDisplay.status} size="md">
                  {dagRunToDisplay.statusLabel || ''}
                </StatusChip>
              </div>
            )}

            {/* Metadata items */}
            <div className="flex flex-wrap items-center gap-4 lg:gap-6 text-sm">
              <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded-lg px-3 py-2">
                <Calendar className="h-4 w-4 text-slate-500" />
                <div className="flex flex-col">
                  <span className="font-medium">
                    {dagRunToDisplay?.startedAt
                      ? dayjs(dagRunToDisplay.startedAt).format(
                          'MMM D, HH:mm:ss'
                        )
                      : '--'}
                  </span>
                  {dagRunToDisplay?.startedAt && (
                    <span className="text-xs text-slate-500 dark:text-slate-400">
                      {dayjs(dagRunToDisplay.startedAt).format('z')}
                    </span>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 rounded-lg px-3 py-2">
                <Timer className="h-4 w-4 text-slate-500" />
                <span className="font-medium">
                  {dagRunToDisplay.finishedAt
                    ? formatDuration(
                        dagRunToDisplay.startedAt,
                        dagRunToDisplay.finishedAt
                      )
                    : dagRunToDisplay.startedAt
                      ? formatDuration(
                          dagRunToDisplay.startedAt,
                          dayjs().toISOString()
                        )
                      : '--'}
                </span>
              </div>

              <div className="flex items-center gap-2 text-slate-600 dark:text-slate-400 ml-auto">
                <span className="font-medium text-xs text-slate-500 dark:text-slate-400 uppercase tracking-wide">
                  Run ID
                </span>
                <code className="bg-slate-200 dark:bg-slate-700 text-slate-800 dark:text-slate-200 px-3 py-1.5 rounded-md text-xs font-mono border">
                  {dagRunToDisplay.rootDAGRunId}
                </code>
              </div>
            </div>
          </div>
        )}

      {/* Parameters - Show if present */}
      {dagRunToDisplay.params && (
        <div className="mt-4 border-t border-slate-200 dark:border-slate-700 pt-4">
          <div className="flex items-center gap-2 mb-2">
            <Terminal className="h-4 w-4 text-slate-500" />
            <span className="text-sm font-semibold text-slate-700 dark:text-slate-300">
              Parameters
            </span>
          </div>
          <div className="bg-slate-100 dark:bg-slate-800 rounded-lg px-4 py-3 font-mono text-sm text-slate-700 dark:text-slate-300 max-h-[120px] overflow-y-auto">
            {dagRunToDisplay.params}
          </div>
        </div>
      )}
    </div>
  );
};

export default DAGHeader;
