import React from 'react';
import { CircularProgress, Container } from '@mui/material';

function LoadingIndicator() {
  return (
    <Container sx={{ width: '100%', textAlign: 'center', margin: 'auto' }}>
      <CircularProgress />
    </Container>
  );
}

export default LoadingIndicator;
