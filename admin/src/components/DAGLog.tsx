import { Box } from '@mui/material';
import React from 'react';
import { LogFile } from '../api/DAG';
import BorderedBox from './BorderedBox';
import Loading from './Loading';
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
      <BorderedBox
        sx={{
          mt: 2,
          py: 2,
          px: 2,
          display: 'flex',
          flexDirection: 'column',
          height: '70vh',
          overflow: 'auto',
        }}
      >
        <pre
          style={{
            backgroundColor: 'black',
            color: 'white',
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
