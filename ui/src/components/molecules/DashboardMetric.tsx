import { Box, Typography } from '@mui/material';
import React from 'react';
import Title from '../atoms/Title';

type Props = {
  title: string;
  color: string | undefined;
  value: string | number;
};

function DashboardMetric({ title, color, value }: Props) {
  return (
    <React.Fragment>
      <Title>{title}</Title>
      <Box
        sx={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          flexGrow: 1,
        }}
      >
        <Typography component="p" variant="h2" color={color}>
          {value}
        </Typography>
      </Box>
    </React.Fragment>
  );
}

export default DashboardMetric;
