/**
 * DAGAttributes component displays the attributes of a DAG.
 *
 * @module features/dags/components/dag-editor
 */
import { Calendar, CheckSquare, Settings, Tag } from 'lucide-react';
import { components } from '../../../../api/v2/schema';
import { Badge } from '../../../../components/ui/badge';

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
    <div>
      <h2 className="text-xl font-semibold text-slate-800 mb-4">
        {dag.name}
      </h2>

      {dag.description && (
        <p className="text-slate-600 mb-6">
          {dag.description}
        </p>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Schedule */}
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-sm font-medium text-slate-600 mb-2">
            <Calendar className="h-4 w-4" />
            <span>Schedule</span>
          </div>

          {!dag.schedule?.length ? (
            <div className="text-sm text-slate-500 italic">
              No schedule defined
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              {dag.schedule?.map((schedule) => (
                <Badge
                  key={schedule.expression}
                  variant="outline"
                  className="bg-blue-50 text-blue-600 border-blue-200 px-2.5 py-1"
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
            <div className="flex items-center gap-2 text-sm font-medium text-slate-600 mb-2">
              <Tag className="h-4 w-4" />
              <span>Parameters</span>
            </div>

            <div className="flex flex-wrap gap-2">
              {dag.params.map((param) => (
                <Badge
                  key={param}
                  variant="outline"
                  className="bg-slate-100 text-slate-700 px-2.5 py-1"
                >
                  {param}
                </Badge>
              ))}
            </div>
          </div>
        )}

        {/* Max Active Runs */}
        {dag.maxActiveSteps && (
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm font-medium text-slate-600 mb-2">
              <Settings className="h-4 w-4" />
              <span>Max Active Runs</span>
            </div>

            <div className="font-medium text-slate-800">
              {dag.maxActiveSteps}
            </div>
          </div>
        )}

        {/* Preconditions */}
        {dag.preconditions && dag.preconditions.length > 0 && (
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-sm font-medium text-slate-600 mb-2">
              <CheckSquare className="h-4 w-4" />
              <span>Preconditions</span>
            </div>

            <div className="space-y-2">
              {dag.preconditions.map((c, index) => (
                <div
                  key={index}
                  className="flex items-center gap-2 text-xs bg-slate-100 rounded-md p-2"
                >
                  <span className="font-medium text-slate-700">
                    {c.condition}
                  </span>
                  <span className="text-slate-500">=&gt;</span>
                  <span className="text-slate-700">
                    {c.expected}
                  </span>
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
