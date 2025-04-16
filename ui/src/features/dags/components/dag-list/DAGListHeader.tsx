import React from 'react';
import Box from '@mui/material/Box';
import Title from '../../../../ui/Title';
import { CreateDAGButton } from '../common';

const DAGListHeader: React.FC = () => (
  <Box
    sx={{
      display: 'flex',
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: 'space-between',
    }}
  >
    <Title>DAGs</Title>
    <CreateDAGButton />
  </Box>
);

export default DAGListHeader;
