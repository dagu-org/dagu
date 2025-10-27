/**
 * DAGStepTableRow component renders a single row in the DAG step table.
 *
 * @module features/dags/components/dag-details
 */
import { CommandDisplay } from '@/components/ui/command-display';
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
  const subDagName = step.call;
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
          {step.id && (
            <div className="text-xs text-slate-600 dark:text-slate-400 font-mono">
              ID: {step.id}
            </div>
          )}
          {step.description && (
            <div className="text-xs text-slate-500 dark:text-slate-400">
              {step.description}
            </div>
          )}
          {subDagName && (
            <div className="mt-1 flex w-fit items-center gap-1.5 rounded-md bg-purple-50 px-1.5 py-0.5 text-xs dark:bg-purple-900/20">
              <GitBranch className="h-3.5 w-3.5 text-purple-500 dark:text-purple-400" />
              <span className="font-medium text-purple-600 dark:text-purple-400">
                Sub-DAG: {subDagName}
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
            <CommandDisplay
              command={step.command || step.cmdWithArgs || ''}
              args={step.command ? step.args : undefined}
              icon="terminal"
              maxLength={50}
            />
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
            <div className="space-y-1">
              <Badge
                variant="outline"
                className={`flex items-center gap-1.5 px-2 py-0.5 text-xs ${
                  step.repeatPolicy.repeat === 'while'
                    ? 'bg-cyan-50 dark:bg-cyan-900/20 text-cyan-600 dark:text-cyan-400 border-cyan-200 dark:border-cyan-800'
                    : step.repeatPolicy.repeat === 'until'
                      ? 'bg-purple-50 dark:bg-purple-900/20 text-purple-600 dark:text-purple-400 border-purple-200 dark:border-purple-800'
                      : 'bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400 border-blue-200 dark:border-blue-800'
                }`}
              >
                <RefreshCw className="h-3 w-3" />
                <span className="font-medium uppercase tracking-wider text-[10px]">
                  {step.repeatPolicy.repeat === 'while'
                    ? 'WHILE'
                    : step.repeatPolicy.repeat === 'until'
                      ? 'UNTIL'
                      : 'REPEAT'}
                </span>
                {step.repeatPolicy.interval && (
                  <span className="text-[10px] opacity-75">
                    {step.repeatPolicy.interval}s
                  </span>
                )}
                {step.repeatPolicy.limit && (
                  <span className="text-[10px] opacity-75">
                    ×{step.repeatPolicy.limit}
                  </span>
                )}
              </Badge>

              {/* Repeat Condition */}
              {step.repeatPolicy.condition && (
                <div className="text-[10px] bg-slate-100 dark:bg-slate-800 rounded px-1.5 py-0.5 font-mono">
                  <span className="text-slate-500 dark:text-slate-400">
                    {step.repeatPolicy.repeat === 'while'
                      ? '↻ while'
                      : '↻ until'}
                    :
                  </span>{' '}
                  <span className="text-slate-700 dark:text-slate-300">
                    {step.repeatPolicy.condition.condition}
                  </span>
                  {step.repeatPolicy.condition.expected && (
                    <>
                      <span className="text-slate-500 dark:text-slate-400">
                        {' '}
                        ={' '}
                      </span>
                      <span className="text-emerald-600 dark:text-emerald-400">
                        {step.repeatPolicy.condition.expected}
                      </span>
                    </>
                  )}
                </div>
              )}

              {/* Exit Codes */}
              {step.repeatPolicy.exitCode &&
                step.repeatPolicy.exitCode.length > 0 && (
                  <div className="text-[10px] bg-slate-100 dark:bg-slate-800 rounded px-1.5 py-0.5">
                    <span className="text-slate-500 dark:text-slate-400">
                      exit codes:
                    </span>{' '}
                    <span className="font-mono text-amber-600 dark:text-amber-400">
                      [{step.repeatPolicy.exitCode.join(', ')}]
                    </span>
                  </div>
                )}
            </div>
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
