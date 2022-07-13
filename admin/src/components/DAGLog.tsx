import { Box, Stack } from '@mui/material';
import React from 'react';
import { LogFile } from '../api/DAG';
import BorderedBox from './BorderedBox';
import LabeledItem from './LabeledItem';
import Loading from './Loading';
import NodeStatusChip from './NodeStatusChip';
import SubTitle from './SubTitle';

type Props = {
  log?: LogFile;
};

function DAGLog({ log }: Props) {
  if (!log) {
    return <Loading />;
  }
  return (
    <Box>
      <SubTitle>Log</SubTitle>
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
        }}
      >
        <pre
          style={{
            backgroundColor: 'black',
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

export default DAGLog;
