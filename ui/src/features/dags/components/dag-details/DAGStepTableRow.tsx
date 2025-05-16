/**
 * DAGStepTableRow component renders a single row in the DAG step table.
 *
 * @module features/dags/components/dag-details
 */
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  ArrowRight,
  FileText,
  Folder,
  GitBranch,
  Mail,
  RefreshCw,
  Terminal,
} from 'lucide-react';
import { components } from '../../../../api/v2/schema';
import { Badge } from '../../../../components/ui/badge';
import { TableCell, TableRow } from '../../../../components/ui/table';

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
      className="flex items-center gap-1.5 mb-1 text-xs bg-slate-100 dark:bg-slate-800 rounded-md p-1.5"
    >
      <span className="font-medium text-slate-700 dark:text-slate-300">
        {c.condition}
      </span>
      <span className="text-slate-500">=&gt;</span>
      <span className="text-slate-700 dark:text-slate-300">{c.expected}</span>
    </div>
  ));

  return (
    <TableRow className="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors duration-200 h-auto">
      {/* Number */}
      <TableCell className="text-center font-semibold text-slate-700 dark:text-slate-300 text-xs py-2">
        {index + 1}
      </TableCell>

      {/* Step Details */}
      <TableCell>
        <div className="space-y-1">
          <div className="text-sm font-semibold text-slate-800 dark:text-slate-200 break-all">
            {step.name}
          </div>
          {step.description && (
            <div className="text-xs text-slate-500 dark:text-slate-400">
              {step.description}
            </div>
          )}
          {step.run && (
            <div className="mt-1 flex w-fit items-center gap-1.5 rounded-md bg-purple-50 px-1.5 py-0.5 text-xs dark:bg-purple-900/20">
              <GitBranch className="h-3.5 w-3.5 text-purple-500 dark:text-purple-400" />
              <span className="font-medium text-purple-600 dark:text-purple-400">
                Sub-DAG: {step.run}
              </span>
            </div>
          )}
        </div>
      </TableCell>

      {/* Execution */}
      <TableCell>
        <div className="space-y-1.5">
          {/* Command & Args */}
          {(step.command || step.cmdWithArgs) && (
            <div className="space-y-1.5">
              <div className="flex items-center gap-1.5 text-xs">
                <Terminal className="h-3.5 w-3.5 text-blue-500 dark:text-blue-400" />
                <span className="bg-slate-100 dark:bg-slate-800 rounded-md px-2 py-0.5 font-medium text-xs text-slate-700 dark:text-slate-300">
                  {step.command || step.cmdWithArgs?.split(' ')[0]}
                </span>
              </div>

              {step.args && step.args.length > 0 && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="pl-5 text-xs text-slate-500 dark:text-slate-400 font-mono truncate cursor-pointer leading-tight">
                      {step.args.join(' ')}
                    </div>
                  </TooltipTrigger>
                  <TooltipContent>
                    <span className="max-w-[400px] break-all font-mono text-xs">
                      {step.args.join(' ')}
                    </span>
                  </TooltipContent>
                </Tooltip>
              )}
            </div>
          )}

          {/* Script */}
          {step.script && (
            <div className="flex items-center gap-1.5 text-xs bg-amber-50 dark:bg-amber-900/10 rounded-md px-1.5 py-0.5 w-fit">
              <FileText className="h-3.5 w-3.5 text-amber-500 dark:text-amber-400" />
              <span className="font-medium text-amber-600 dark:text-amber-400">
                Script defined
              </span>
            </div>
          )}

          {/* Directory */}
          {step.dir && (
            <div className="flex items-center gap-1.5 text-xs bg-slate-50 dark:bg-slate-800/50 rounded-md px-1.5 py-0.5 w-fit">
              <Folder className="h-3.5 w-3.5 text-slate-500 dark:text-slate-400" />
              <span className="font-medium text-slate-600 dark:text-slate-400">
                {step.dir}
              </span>
            </div>
          )}

          {/* Output */}
          {step.output && (
            <div className="flex items-center gap-1.5 text-xs bg-green-50 dark:bg-green-900/10 rounded-md px-1.5 py-0.5 w-fit">
              <ArrowRight className="h-3.5 w-3.5 text-green-500 dark:text-green-400" />
              <span className="font-medium text-green-600 dark:text-green-400">
                Output: {step.output}
              </span>
            </div>
          )}
        </div>
      </TableCell>

      {/* Dependencies */}
      <TableCell>
        {step.depends && step.depends.length > 0 ? (
          <div className="flex flex-wrap gap-1">
            {step.depends.map((dep, idx) => (
              <Badge
                key={idx}
                variant="outline"
                className="bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-300 px-1.5 py-0.5 text-wrap break-all text-xs"
              >
                {dep}
              </Badge>
            ))}
          </div>
        ) : (
          <span className="text-xs text-slate-500 leading-tight">None</span>
        )}
      </TableCell>

      {/* Configuration */}
      <TableCell>
        <div className="space-y-1.5">
          {/* Repeat Policy */}
          {step.repeatPolicy?.repeat && (
            <Badge
              variant="outline"
              className="flex items-center gap-1.5 bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-200 dark:border-blue-800 px-2 py-0.5 text-xs"
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
            <div className="flex items-center gap-1.5 text-xs bg-red-50 dark:bg-red-900/10 rounded-md px-1.5 py-0.5 w-fit">
              <Mail className="h-3.5 w-3.5 text-red-500 dark:text-red-400" />
              <span className="font-medium text-red-600 dark:text-red-400">
                Mail on Error
              </span>
            </div>
          )}

          {/* Stdout/Stderr */}
          {(step.stdout || step.stderr) && (
            <div className="text-xs text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/50 rounded-md p-1.5 leading-tight">
              {step.stdout && (
                <div className="mb-1">
                  stdout: <span className="font-mono">{step.stdout}</span>
                </div>
              )}
              {step.stderr && (
                <div>
                  stderr: <span className="font-mono">{step.stderr}</span>
                </div>
              )}
            </div>
          )}

          {/* Params for Sub-DAG */}
          {step.params && (
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="text-xs text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/50 rounded-md p-1.5 truncate cursor-pointer leading-tight">
                  <span className="font-medium">Params:</span>{' '}
                  <span className="font-mono">{step.params}</span>
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <span className="max-w-[400px] break-all font-mono text-xs">
                  {step.params}
                </span>
              </TooltipContent>
            </Tooltip>
          )}
        </div>
      </TableCell>

      {/* Preconditions */}
      <TableCell>
        {preconditions && preconditions.length > 0 ? (
          <div className="space-y-1">{preconditions}</div>
        ) : (
          <span className="text-xs text-slate-500 leading-tight">None</span>
        )}
      </TableCell>
    </TableRow>
  );
}

export default DAGStepTableRow;
