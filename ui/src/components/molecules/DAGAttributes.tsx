import React from 'react';
import { Stack, Box, Chip } from '@mui/material';
import LabeledItem from '../atoms/LabeledItem';
import { DAG } from '../../models';

type Props = {
  dag: DAG;
};

function DAGAttributes({ dag: config }: Props) {
  const preconditions = config.Preconditions?.map((c) => (
    <li>
      {c.Condition}
      {' => '}
      {c.Expected}
    </li>
  ));
  return (
    <Stack direction="column" spacing={1}>
      <LabeledItem label="Name">{config.Name}</LabeledItem>
      <LabeledItem label="Schedule">
        <Stack direction={'row'}>
          {config.Schedule?.map((s) => (
            <Chip
              key={s.Expression}
              sx={{
                fontWeight: 'semibold',
                marginRight: 1,
              }}
              size="small"
              label={s.Expression}
            />
          ))}
        </Stack>
      </LabeledItem>
      <LabeledItem label="Description">{config.Description}</LabeledItem>
      <LabeledItem label="Max Active Runs">{config.MaxActiveRuns}</LabeledItem>
      <LabeledItem label="Params">{config.Params?.join(' ')}</LabeledItem>
      <Stack direction={'column'}>
        <React.Fragment>
          <LabeledItem label="Preconditions">{null}</LabeledItem>
          <Box sx={{ pl: 2 }}>
            <ul>{preconditions}</ul>
          </Box>
        </React.Fragment>
      </Stack>
    </Stack>
  );
}

export default DAGAttributes;
