import React from 'react';
import { LogFile } from '../api/DAG';
import BorderedBox from './BorderedBox';
import Loading from './Loading';

type Props = {
  log?: LogFile;
};

function DAGLog({ log }: Props) {
  if (!log) {
    return <Loading />;
  }
  return (
    <BorderedBox
      sx={{
        pb: 4,
        px: 2,
        mx: 4,
        display: 'flex',
        flexDirection: 'column',
        height: '70vh',
        overflow: 'auto',
        borderTopWidth: 0,
        borderTopLeftRadius: 0,
        borderTopRightRadius: 0,
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
  );
}

export default DAGLog;
