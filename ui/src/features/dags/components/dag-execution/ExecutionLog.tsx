/**
 * ExecutionLog component displays the execution log for a DAG run.
 *
 * @module features/dags/components/dag-execution
 */
import React from 'react';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';

/**
 * Props for the ExecutionLog component
 */
type Props = {
  /** DAG name or fileName */
  name: string;
  /** Request ID of the execution */
  requestId: string;
};

/**
 * Regular expression to match ANSI color codes for removal
 * Credit: https://github.com/chalk/ansi-regex/commit/02fa893d619d3da85411acc8fd4e2eea0e95a9d9 under MIT license
 */
const ANSI_CODES_REGEX = [
  '[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\\u0007)',
  '(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-nq-uy=><~]))',
].join('|');

/**
 * ExecutionLog displays the log output for a DAG run
 * Fetches log data from the API and refreshes every 30 seconds
 */
function ExecutionLog({ name, requestId }: Props) {
  const appBarContext = React.useContext(AppBarContext);

  // Fetch log data with periodic refresh
  const { data } = useQuery(
    '/runs/{dagName}/{requestId}/log',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
        path: {
          dagName: name,
          requestId,
        },
      },
    },
    { refreshInterval: 30000 } // Refresh every 30 seconds
  );

  // Show loading indicator while fetching data
  if (!data) {
    return <LoadingIndicator />;
  }

  // Remove ANSI color codes from log content
  const content = data.content.replace(new RegExp(ANSI_CODES_REGEX, 'g'), '');

  // Split content into lines for better rendering
  const lines = content ? content.split('\n') : ['<No log output>'];

  return (
    <div className="w-full h-full">
      <div className="h-full overflow-auto rounded-lg bg-zinc-900 p-4 shadow-md">
        <pre className="h-full font-mono text-sm text-white">
          {lines.map((line, index) => (
            <div
              key={index}
              className="flex hover:bg-zinc-800 px-2 py-0.5 rounded"
            >
              <span className="text-zinc-500 mr-4 select-none w-8 text-right">
                {index + 1}
              </span>
              <span className="whitespace-pre-wrap break-all">
                {line || ' '}
              </span>
            </div>
          ))}
        </pre>
      </div>
    </div>
  );
}

export default ExecutionLog;
