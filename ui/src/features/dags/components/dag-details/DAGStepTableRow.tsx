/**
 * DAGStepTableRow component renders a single row in the DAG step table.
 *
 * @module features/dags/components/dag-details
 */
import { CommandDisplay } from '@/components/ui/command-display';
import { ScriptBadge } from '@/components/ui/script-dialog';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { ArrowRight, Code, Folder, GitBranch, Mail, RefreshCw } from 'lucide-react';
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
 * Get displayable command from executor config when step.commands is empty
 */
const getExecutorCommand = (
  step: components['schemas']['Step']
): string | null => {
  const type = step.executorConfig?.type;
  const config = step.executorConfig?.config as Record<string, unknown>;

  if (!type || !config) return null;

  switch (type) {
    case 'redis':
      if (config.command) {
        const parts = [config.command as string];
        if (config.key) parts.push(config.key as string);
        return parts.join(' ');
      }
      return null;
    case 'sql':
      return config.query ? String(config.query) : null;
    case 'http':
      return config.url ? `${config.method || 'GET'} ${config.url}` : null;
    case 'mail':
      return config.to ? `Mail to ${config.to}` : null;
    case 'jq':
      return config.expression ? `jq: ${config.expression}` : null;
    case 'docker':
      return config.image ? `docker: ${config.image}` : null;
    default:
      return null;
  }
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
      className="flex items-center gap-1.5 mb-1 text-xs bg-muted rounded-md p-1.5"
    >
      <span className="font-medium text-foreground/90">
        {c.condition}
      </span>
      <span className="text-muted-foreground">=&gt;</span>
      <span className="text-foreground/90">{c.expected}</span>
    </div>
  ));

  return (
    <TableRow className="hover:bg-muted transition-colors duration-200 h-auto">
      {/* Number */}
      <TableCell className="text-center font-semibold text-foreground/90 text-xs py-2">
        {index + 1}
      </TableCell>

      {/* Step Details */}
      <TableCell>
        <div className="space-y-1">
          <div className="text-sm font-semibold text-foreground break-all">
            {step.name}
          </div>
          {step.id && (
            <div className="text-xs text-muted-foreground font-mono">
              ID: {step.id}
            </div>
          )}
          {step.description && (
            <div className="text-xs text-muted-foreground">
              {step.description}
            </div>
          )}
          {subDagName && (
            <div className="mt-1 flex w-fit items-center gap-1.5 rounded-md bg-info-muted px-1.5 py-0.5 text-xs">
              <GitBranch className="h-3.5 w-3.5 text-info" />
              <span className="font-medium text-info">
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
          {step.commands && step.commands.length > 0 ? (
            <CommandDisplay
              commands={step.commands}
              icon="terminal"
              maxLength={50}
            />
          ) : (
            // Executor-specific display
            (() => {
              const execCmd = getExecutorCommand(step);
              if (execCmd) {
                return (
                  <div className="flex items-center gap-1.5 text-sm text-muted-foreground">
                    <Code className="h-3.5 w-3.5 text-primary flex-shrink-0" />
                    <span
                      className="font-mono text-xs truncate max-w-[200px]"
                      title={execCmd}
                    >
                      {execCmd.length > 50
                        ? execCmd.slice(0, 47) + '...'
                        : execCmd}
                    </span>
                  </div>
                );
              }
              return null;
            })()
          )}

          {/* Script */}
          {step.script && (
            <ScriptBadge script={step.script} stepName={step.name} />
          )}

          {/* Directory */}
          {step.dir && (
            <div className="flex items-center gap-1.5 text-xs bg-muted rounded-md px-1.5 py-0.5 w-fit">
              <Folder className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="font-medium text-muted-foreground">
                {step.dir}
              </span>
            </div>
          )}

          {/* Output */}
          {step.output && (
            <div className="flex items-center gap-1.5 text-xs bg-success-muted rounded-md px-1.5 py-0.5 w-fit">
              <ArrowRight className="h-3.5 w-3.5 text-success" />
              <span className="font-medium text-success">
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
                className="bg-muted text-foreground/90 px-1.5 py-0.5 text-wrap break-all text-xs"
              >
                {dep}
              </Badge>
            ))}
          </div>
        ) : (
          <span className="text-xs text-muted-foreground leading-tight">None</span>
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
                    ? 'bg-info-muted text-info border-info/30'
                    : step.repeatPolicy.repeat === 'until'
                      ? 'bg-info-muted text-info border-info/30'
                      : 'bg-primary/10 text-primary border-primary/30'
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
                <div className="text-[10px] bg-muted rounded px-1.5 py-0.5 font-mono">
                  <span className="text-muted-foreground">
                    {step.repeatPolicy.repeat === 'while'
                      ? '↻ while'
                      : '↻ until'}
                    :
                  </span>{' '}
                  <span className="text-foreground/90">
                    {step.repeatPolicy.condition.condition}
                  </span>
                  {step.repeatPolicy.condition.expected && (
                    <>
                      <span className="text-muted-foreground">
                        {' '}
                        ={' '}
                      </span>
                      <span className="text-success">
                        {step.repeatPolicy.condition.expected}
                      </span>
                    </>
                  )}
                </div>
              )}

              {/* Exit Codes */}
              {step.repeatPolicy.exitCode &&
                step.repeatPolicy.exitCode.length > 0 && (
                  <div className="text-[10px] bg-muted rounded px-1.5 py-0.5">
                    <span className="text-muted-foreground">
                      exit codes:
                    </span>{' '}
                    <span className="font-mono text-warning">
                      [{step.repeatPolicy.exitCode.join(', ')}]
                    </span>
                  </div>
                )}
            </div>
          )}

          {/* Mail on Error */}
          {step.mailOnError && (
            <div className="flex items-center gap-1.5 text-xs bg-error-muted rounded-md px-1.5 py-0.5 w-fit">
              <Mail className="h-3.5 w-3.5 text-error" />
              <span className="font-medium text-error">
                Mail on Error
              </span>
            </div>
          )}

          {/* Stdout/Stderr */}
          {(step.stdout || step.stderr) && (
            <div className="text-xs text-muted-foreground bg-muted rounded-md p-1.5 leading-tight">
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
                <div className="text-xs text-muted-foreground bg-muted rounded-md p-1.5 truncate cursor-pointer leading-tight">
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
          <span className="text-xs text-muted-foreground leading-tight">None</span>
        )}
      </TableCell>
    </TableRow>
  );
}

export default DAGStepTableRow;
