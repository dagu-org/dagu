import { Box, Stack } from '@mui/material';
import React from 'react';
import { LogFile } from '../../models/api';
import BorderedBox from '../atoms/BorderedBox';
import LabeledItem from '../atoms/LabeledItem';
import LoadingIndicator from '../atoms/LoadingIndicator';
import NodeStatusChip from '../molecules/NodeStatusChip';

type Props = {
  log?: LogFile;
};

// Credit: https://github.com/chalk/ansi-regex/commit/02fa893d619d3da85411acc8fd4e2eea0e95a9d9 under MIT license
const ANSI_CODES_REGEX = [
  '[\\u001B\\u009B][[\\]()#;?]*(?:(?:(?:(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]+)*|[a-zA-Z\\d]+(?:;[-a-zA-Z\\d\\/#&.:=?%@~_]*)*)?\\u0007)',
  '(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PR-TZcf-nq-uy=><~]))',
].join('|');

function ExecutionLog({ log }: Props) {
  if (!log) {
    return <LoadingIndicator />;
  }
  log.Content = log.Content.replace(new RegExp(ANSI_CODES_REGEX, 'g'), '');
  return (
    <Box>
      <Stack spacing={1} direction="column" sx={{ width: '100%' }}>
        <LabeledItem label="Log File">{log.LogFile}</LabeledItem>
        {log.Step ? (
          <React.Fragment>
            <LabeledItem label="Step Name">{log.Step.Step.Name}</LabeledItem>
            <Stack spacing={2} direction="row" sx={{ alignItems: 'center' }}>
              <LabeledItem label="Started At">{log.Step.StartedAt}</LabeledItem>
              <LabeledItem label="Finished At">
                {log.Step.FinishedAt}
              </LabeledItem>
            </Stack>
            <LabeledItem label="Status">
              <NodeStatusChip status={log.Step.Status}>
                {log.Step.StatusText}
              </NodeStatusChip>
            </LabeledItem>
          </React.Fragment>
        ) : null}
      </Stack>
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
          {log.Content || '<No log output>'}
        </pre>
      </BorderedBox>
    </Box>
  );
}

export default ExecutionLog;
