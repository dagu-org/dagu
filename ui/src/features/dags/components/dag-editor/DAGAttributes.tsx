/**
 * DAGAttributes component displays the attributes of a DAG.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { Badge } from '../../../../components/ui/badge';
import { cn } from '../../../../lib/utils';
import { Calendar, Tag, Settings, CheckSquare } from 'lucide-react';

/**
 * Props for the DAGAttributes component
 */
type Props = {
  /** DAG details to display */
  dag: components['schemas']['DAGDetails'];
};

/**
 * DAGAttributes displays the metadata and configuration of a DAG
 * including name, schedule, description, and other properties
 */
function DAGAttributes({ dag }: Props) {
  return (
    <div className="mb-6 border border-slate-200 dark:border-slate-700 rounded-md p-4 bg-white dark:bg-slate-900">
      <h2 className="text-xl font-semibold mb-4">{dag.name}</h2>

      {dag.description && (
        <p className="text-slate-600 dark:text-slate-300 mb-4">
          {dag.description}
        </p>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Schedule */}
        <div className="space-y-1">
          <div className="flex items-center gap-1.5 text-sm text-slate-500">
            <Calendar className="h-4 w-4" />
            <span>Schedule</span>
          </div>

          {!dag.schedule?.length ? (
            <div className="text-sm text-slate-500">No schedule defined</div>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {dag.schedule?.map((schedule) => (
                <Badge
                  key={schedule.expression}
                  variant="outline"
                  className="bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-200 dark:border-blue-800"
                >
                  {schedule.expression}
                </Badge>
              ))}
            </div>
          )}
        </div>

        {/* Parameters */}
        {dag.params && dag.params.length > 0 && (
          <div className="space-y-1">
            <div className="flex items-center gap-1.5 text-sm text-slate-500">
              <Tag className="h-4 w-4" />
              <span>Parameters</span>
            </div>

            <div className="flex flex-wrap gap-1.5">
              {dag.params.map((param) => (
                <Badge
                  key={param}
                  variant="outline"
                  className="bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300"
                >
                  {param}
                </Badge>
              ))}
            </div>
          </div>
        )}

        {/* Max Active Runs */}
        {dag.maxActiveRuns && (
          <div className="space-y-1">
            <div className="flex items-center gap-1.5 text-sm text-slate-500">
              <Settings className="h-4 w-4" />
              <span>Max Active Runs</span>
            </div>

            <div className="font-medium">{dag.maxActiveRuns}</div>
          </div>
        )}

        {/* Preconditions */}
        {dag.preconditions && dag.preconditions.length > 0 && (
          <div className="space-y-1">
            <div className="flex items-center gap-1.5 text-sm text-slate-500">
              <CheckSquare className="h-4 w-4" />
              <span>Preconditions</span>
            </div>

            <div className="space-y-1">
              {dag.preconditions.map((c, index) => (
                <div
                  key={index}
                  className="flex items-center gap-1 text-xs bg-slate-100 dark:bg-slate-800 rounded p-1"
                >
                  <span className="font-medium">{c.condition}</span>
                  <span className="text-slate-500">=&gt;</span>
                  <span>{c.expected}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default DAGAttributes;
