import { Box, BoxProps } from '@mui/material';
import React from 'react';

interface BorderedBoxProps extends BoxProps {
  children?: React.ReactNode;
}
export default function BorderedBox({
  children,
  sx,
  ...props
}: BorderedBoxProps) {
  return (
    <Box
      sx={{
        border: 1,
        borderColor: '#e0e0e0',
        backgroundColor: '#fff',
        ...sx,
      }}
      {...props}
    >
      {children}
    </Box>
  );
}
