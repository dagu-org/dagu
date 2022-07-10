import React, { CSSProperties } from 'react';
import { Config } from '../models/Config';
import MultilineText from './MultilineText';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from './BorderedBox';

type Props = {
  config: Config;
};

function ConfigInfoTable({ config }: Props) {
  const tableStyle: CSSProperties = {
    tableLayout: 'fixed',
    wordWrap: 'break-word',
  };
  const styles = configTabColStyles;
  const preconditions = config.Preconditions.map((c) => (
    <li>
      {c.Condition}
      {' => '}
      {c.Expected}
    </li>
  ));
  let i = 0;
  return (
    <BorderedBox>
      <Table sx={tableStyle}>
        <TableHead>
          <TableRow>
            <TableCell style={styles[i++]}>Name</TableCell>
            <TableCell style={styles[i++]}>Description</TableCell>
            <TableCell style={styles[i++]}>MaxActiveRuns</TableCell>
            <TableCell style={styles[i++]}>Params</TableCell>
            <TableCell style={styles[i++]}>Preconditions</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          <TableRow>
            <TableCell> {config.Name} </TableCell>
            <TableCell>
              <MultilineText>{config.Description}</MultilineText>
            </TableCell>
            <TableCell> {config.MaxActiveRuns} </TableCell>
            <TableCell> {config.DefaultParams} </TableCell>
            <TableCell>
              <ul>{preconditions}</ul>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </BorderedBox>
  );
}

export default ConfigInfoTable;

const configTabColStyles = [
  { width: '200px' },
  { width: '200px' },
  { width: '150px' },
  { width: '150px' },
  {},
];
