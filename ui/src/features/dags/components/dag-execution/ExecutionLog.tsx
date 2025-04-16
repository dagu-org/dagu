/**
 * ExecutionLog component displays the execution log for a DAG run.
 *
 * @module features/dags/components/dag-execution
 */
import { Box } from '@mui/material';
import React from 'react';
import BorderedBox from '../../../../ui/BorderedBox';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { useQuery } from '../../../../hooks/api';
import { AppBarContext } from '../../../../contexts/AppBarContext';

/**
 * Props for the ExecutionLog component
 */
type Props = {
  /** DAG name/fileId */
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

  return (
    <Box>
      <BorderedBox
        sx={{
          mt: 2,
          py: 2,
          px: 2,
          height: '60vh',
          overflow: 'auto',
          backgroundColor: 'black',
        }}
      >
        <pre
          style={{
            color: 'white',
            height: '100%',
            fontFamily: 'Courier New, Courier, monospace',
          }}
        >
          {content || '<No log output>'}
        </pre>
      </BorderedBox>
    </Box>
  );
}

export default ExecutionLog;
