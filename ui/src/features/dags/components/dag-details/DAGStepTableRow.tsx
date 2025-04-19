/**
 * DAGStepTableRow component renders a single row in the DAG step table.
 *
 * @module features/dags/components/dag-details
 */
import React from 'react';
import { TableCell, TableRow } from '../../../../components/ui/table';
import { components } from '../../../../api/v2/schema';
import { Badge } from '../../../../components/ui/badge';
import { cn } from '../../../../lib/utils';
import {
  Code,
  Folder,
  RefreshCw,
  FileCheck,
  ArrowRight,
  Mail,
  FileText,
  Terminal,
  GitBranch,
} from 'lucide-react';

/**
 * Props for the DAGStepTableRow component
 */
type Props = {
  /** Step definition to display */
  step: components['schemas']['Step'];
  /** Index of the step in the list */
  index: number;
};

/**
 * DAGStepTableRow displays information about a single step in a DAG
 */
function DAGStepTableRow({ step, index }: Props) {
  // Format preconditions as a list
  const preconditions = step.preconditions?.map((c, index) => (
    <div
      key={index}
      className="flex items-center gap-1 mb-1 text-xs bg-slate-100 dark:bg-slate-800 rounded p-1"
    >
      <span className="font-medium">{c.condition}</span>
      <span className="text-slate-500">=&gt;</span>
      <span>{c.expected}</span>
    </div>
  ));

  return (
    <TableRow className="hover:bg-slate-50 dark:hover:bg-slate-800/50">
      {/* Number */}
      <TableCell className="text-center font-semibold">{index + 1}</TableCell>

      {/* Step Details */}
      <TableCell>
        <div className="space-y-1">
          <div className="font-semibold">{step.name}</div>
          {step.description && (
            <div className="text-sm text-slate-500">{step.description}</div>
          )}
          {step.run && (
            <div className="flex items-center gap-1 text-xs mt-1">
              <GitBranch className="h-3.5 w-3.5 text-purple-500" />
              <span className="font-medium text-purple-600 dark:text-purple-400">
                Sub-DAG: {step.run}
              </span>
            </div>
          )}
        </div>
      </TableCell>

      {/* Execution */}
      <TableCell>
        <div className="space-y-2">
          {/* Command & Args */}
          {(step.command || step.cmdWithArgs) && (
            <div className="space-y-1">
              <div className="flex items-center gap-1 text-sm">
                <Terminal className="h-3.5 w-3.5 text-blue-500" />
                <span className="bg-slate-100 dark:bg-slate-800 rounded px-1 py-0.5 font-medium text-xs">
                  {step.command || step.cmdWithArgs?.split(' ')[0]}
                </span>
              </div>

              {step.args && step.args.length > 0 && (
                <div className="pl-5 text-xs text-slate-500 truncate">
                  {step.args.join(' ')}
                </div>
              )}
            </div>
          )}

          {/* Script */}
          {step.script && (
            <div className="flex items-center gap-1 text-xs">
              <FileText className="h-3.5 w-3.5 text-amber-500" />
              <span className="font-medium">Script defined</span>
            </div>
          )}

          {/* Directory */}
          {step.dir && (
            <div className="flex items-center gap-1 text-xs">
              <Folder className="h-3.5 w-3.5 text-slate-400" />
              <span className="font-medium">{step.dir}</span>
            </div>
          )}

          {/* Output */}
          {step.output && (
            <div className="flex items-center gap-1 text-xs">
              <ArrowRight className="h-3.5 w-3.5 text-green-500" />
              <span className="font-medium">Output: {step.output}</span>
            </div>
          )}
        </div>
      </TableCell>

      {/* Dependencies */}
      <TableCell>
        {step.depends && step.depends.length > 0 ? (
          <div className="space-y-1">
            {step.depends.map((dep, idx) => (
              <Badge
                key={idx}
                variant="outline"
                className="mr-1 mb-1 bg-slate-100 dark:bg-slate-800"
              >
                {dep}
              </Badge>
            ))}
          </div>
        ) : (
          <span className="text-xs text-slate-500">None</span>
        )}
      </TableCell>

      {/* Configuration */}
      <TableCell>
        <div className="space-y-2">
          {/* Repeat Policy */}
          {step.repeatPolicy?.repeat && (
            <Badge
              variant="outline"
              className="flex items-center gap-1 bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-200 dark:border-blue-800"
            >
              <RefreshCw className="h-3 w-3" />
              <span>
                Repeat
                {step.repeatPolicy.interval
                  ? ` (${step.repeatPolicy.interval}s)`
                  : ''}
              </span>
            </Badge>
          )}

          {/* Mail on Error */}
          {step.mailOnError && (
            <div className="flex items-center gap-1 text-xs">
              <Mail className="h-3.5 w-3.5 text-red-500" />
              <span className="font-medium">Mail on Error</span>
            </div>
          )}

          {/* Stdout/Stderr */}
          {(step.stdout || step.stderr) && (
            <div className="text-xs text-slate-500">
              {step.stdout && <div>stdout: {step.stdout}</div>}
              {step.stderr && <div>stderr: {step.stderr}</div>}
            </div>
          )}

          {/* Params for Sub-DAG */}
          {step.params && (
            <div className="text-xs text-slate-500 truncate">
              <span className="font-medium">Params:</span> {step.params}
            </div>
          )}
        </div>
      </TableCell>

      {/* Preconditions */}
      <TableCell>
        {preconditions && preconditions.length > 0 ? (
          <div className="space-y-1">{preconditions}</div>
        ) : (
          <span className="text-xs text-slate-500">None</span>
        )}
      </TableCell>
    </TableRow>
  );
}

export default DAGStepTableRow;
