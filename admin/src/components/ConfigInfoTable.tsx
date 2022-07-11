import React from 'react';
import { Config } from '../models/Config';
import { Stack, Box } from '@mui/material';
import LabeledItem from './LabeledItem';

type Props = {
  config: Config;
};

function ConfigInfoTable({ config }: Props) {
  const preconditions = config.Preconditions.map((c) => (
    <li>
      {c.Condition}
      {' => '}
      {c.Expected}
    </li>
  ));
  return (
    <Stack direction="column" spacing={1}>
      <LabeledItem label="Name">{config.Name}</LabeledItem>
      <LabeledItem label="Description">{config.Description}</LabeledItem>
      <LabeledItem label="Max Active Runs">{config.MaxActiveRuns}</LabeledItem>
      <LabeledItem label="Params">{config.Params}</LabeledItem>
      <Stack direction={'column'}>
        <React.Fragment>
          <LabeledItem label="Pre Conditions">{null}</LabeledItem>
          <Box sx={{ pl: 2 }}>
            <ul>{preconditions}</ul>
          </Box>
        </React.Fragment>
      </Stack>
    </Stack>
  );
}

export default ConfigInfoTable;
