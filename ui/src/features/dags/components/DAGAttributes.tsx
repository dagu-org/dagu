import React from 'react';
import { Stack, Box, Chip } from '@mui/material';
import LabeledItem from '../../../ui/LabeledItem';
import { components } from '../../../api/v2/schema';

type Props = {
  dag: components['schemas']['DAGDetails'];
};

function DAGAttributes({ dag }: Props) {
  const preconditions = dag.preconditions?.map((c) => (
    <li>
      {c.condition}
      {' => '}
      {c.expected}
    </li>
  ));
  return (
    <Stack direction="column" spacing={1}>
      <LabeledItem label="Name">{dag.name}</LabeledItem>
      <LabeledItem label="Schedule">
        {!dag.schedule?.length && 'No schedule'}
        <Stack direction={'row'}>
          {dag.schedule?.map((schedule) => (
            <Chip
              key={schedule.expression}
              sx={{
                fontWeight: 'semibold',
                marginRight: 1,
              }}
              size="small"
              label={schedule.expression}
            />
          ))}
        </Stack>
      </LabeledItem>
      <LabeledItem label="Description">{dag.description}</LabeledItem>
      {dag.maxActiveRuns ? (
        <LabeledItem label="Max Active Runs">{dag.maxActiveRuns}</LabeledItem>
      ) : null}
      <LabeledItem label="Params">{dag.params?.join(' ')}</LabeledItem>
      {preconditions?.length ? (
        <Stack direction={'column'}>
          <React.Fragment>
            <LabeledItem label="Preconditions">{null}</LabeledItem>
            <Box sx={{ pl: 2 }}>
              <ul>{preconditions}</ul>
            </Box>
          </React.Fragment>
        </Stack>
      ) : null}
    </Stack>
  );
}

export default DAGAttributes;
